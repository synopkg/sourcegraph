package campaigns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/sourcegraph/src-cli/internal/api"
	"github.com/sourcegraph/src-cli/internal/campaigns/docker"
	"github.com/sourcegraph/src-cli/internal/campaigns/graphql"
)

type Service struct {
	allowUnsupported bool
	client           api.Client
	features         featureFlags
	imageCache       *docker.ImageCache
	workspace        string
}

type ServiceOpts struct {
	AllowUnsupported bool
	Client           api.Client
	Workspace        string
}

var (
	ErrMalformedOnQueryOrRepository = errors.New("malformed 'on' field; missing either a repository name or a query")
)

func NewService(opts *ServiceOpts) *Service {
	return &Service{
		allowUnsupported: opts.AllowUnsupported,
		client:           opts.Client,
		imageCache:       docker.NewImageCache(),
		workspace:        opts.Workspace,
	}
}

const sourcegraphVersionQuery = `query SourcegraphVersion {
	site {
	  productVersion
	}
  }
  `

// getSourcegraphVersion queries the Sourcegraph GraphQL API to get the
// current version of the Sourcegraph instance.
func (svc *Service) getSourcegraphVersion(ctx context.Context) (string, error) {
	var result struct {
		Site struct {
			ProductVersion string
		}
	}

	ok, err := svc.client.NewQuery(sourcegraphVersionQuery).Do(ctx, &result)
	if err != nil || !ok {
		return "", err
	}

	return result.Site.ProductVersion, err
}

// DetermineFeatureFlags fetches the version of the configured Sourcegraph
// instance and then sets flags on the Service itself to use features available
// in that version, e.g. gzip compression.
func (svc *Service) DetermineFeatureFlags(ctx context.Context) error {
	version, err := svc.getSourcegraphVersion(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to query Sourcegraph version to check for available features")
	}

	return svc.features.setFromVersion(version)
}

func (svc *Service) newRequest(query string, vars map[string]interface{}) api.Request {
	if svc.features.useGzipCompression {
		return svc.client.NewGzippedRequest(query, vars)
	}
	return svc.client.NewRequest(query, vars)
}

type CampaignSpecID string
type ChangesetSpecID string

const applyCampaignMutation = `
mutation ApplyCampaign($campaignSpec: ID!) {
    applyCampaign(campaignSpec: $campaignSpec) {
        ...campaignFields
    }
}
` + graphql.CampaignFieldsFragment

func (svc *Service) ApplyCampaign(ctx context.Context, spec CampaignSpecID) (*graphql.Campaign, error) {
	var result struct {
		Campaign *graphql.Campaign `json:"applyCampaign"`
	}
	if ok, err := svc.newRequest(applyCampaignMutation, map[string]interface{}{
		"campaignSpec": spec,
	}).Do(ctx, &result); err != nil || !ok {
		return nil, err
	}
	return result.Campaign, nil
}

const createCampaignSpecMutation = `
mutation CreateCampaignSpec(
    $namespace: ID!,
    $spec: String!,
    $changesetSpecs: [ID!]!
) {
    createCampaignSpec(
        namespace: $namespace, 
        campaignSpec: $spec,
        changesetSpecs: $changesetSpecs
    ) {
        id
        applyURL
    }
}
`

func (svc *Service) CreateCampaignSpec(ctx context.Context, namespace, spec string, ids []ChangesetSpecID) (CampaignSpecID, string, error) {
	var result struct {
		CreateCampaignSpec struct {
			ID       string
			ApplyURL string
		}
	}
	if ok, err := svc.client.NewRequest(createCampaignSpecMutation, map[string]interface{}{
		"namespace":      namespace,
		"spec":           spec,
		"changesetSpecs": ids,
	}).Do(ctx, &result); err != nil || !ok {
		return "", "", err
	}

	return CampaignSpecID(result.CreateCampaignSpec.ID), result.CreateCampaignSpec.ApplyURL, nil

}

const createChangesetSpecMutation = `
mutation CreateChangesetSpec($spec: String!) {
    createChangesetSpec(changesetSpec: $spec) {
        ... on HiddenChangesetSpec {
            id
        }
        ... on VisibleChangesetSpec {
            id
        }
    }
}
`

func (svc *Service) CreateChangesetSpec(ctx context.Context, spec *ChangesetSpec) (ChangesetSpecID, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return "", errors.Wrap(err, "marshalling changeset spec JSON")
	}

	var result struct {
		CreateChangesetSpec struct {
			ID string
		}
	}
	if ok, err := svc.newRequest(createChangesetSpecMutation, map[string]interface{}{
		"spec": string(raw),
	}).Do(ctx, &result); err != nil || !ok {
		return "", err
	}

	return ChangesetSpecID(result.CreateChangesetSpec.ID), nil
}

func (svc *Service) NewExecutionCache(dir string) ExecutionCache {
	if dir == "" {
		return &ExecutionNoOpCache{}
	}

	return &ExecutionDiskCache{dir}
}

func (svc *Service) NewRepoFetcher(dir string, cleanArchives bool) RepoFetcher {
	return &repoFetcher{
		client:     svc.client,
		dir:        dir,
		deleteZips: cleanArchives,
	}
}

func (svc *Service) NewWorkspaceCreator(ctx context.Context, cacheDir, tempDir string, steps []Step) WorkspaceCreator {
	if svc.workspaceCreatorType(ctx, steps) == workspaceCreatorVolume {
		return &dockerVolumeWorkspaceCreator{tempDir: tempDir}
	}
	return &dockerBindWorkspaceCreator{dir: cacheDir}
}

func (svc *Service) workspaceCreatorType(ctx context.Context, steps []Step) workspaceCreatorType {
	if svc.workspace == "volume" {
		return workspaceCreatorVolume
	} else if svc.workspace == "bind" {
		return workspaceCreatorBind
	}

	return bestWorkspaceCreator(ctx, steps)
}

// SetDockerImages updates the steps within the campaign spec to include the
// exact content digest to be used when running each step, and ensures that all
// Docker images are available, including any required by the service itself.
//
// Progress information is reported back to the given progress function: perc
// will be a value between 0.0 and 1.0, inclusive.
func (svc *Service) SetDockerImages(ctx context.Context, spec *CampaignSpec, progress func(perc float64)) error {
	total := len(spec.Steps) + 1
	progress(0)

	// TODO: this _really_ should be parallelised, since the image cache takes
	// care to only pull the same image once.
	for i := range spec.Steps {
		spec.Steps[i].image = svc.imageCache.Get(spec.Steps[i].Container)
		if err := spec.Steps[i].image.Ensure(ctx); err != nil {
			return errors.Wrapf(err, "pulling image %q", spec.Steps[i].Container)
		}
		progress(float64(i) / float64(total))
	}

	// We also need to ensure we have our own utility images available, if
	// necessary.
	if svc.workspaceCreatorType(ctx, spec.Steps) == workspaceCreatorVolume {
		if err := svc.imageCache.Get(dockerVolumeWorkspaceImage).Ensure(ctx); err != nil {
			return errors.Wrapf(err, "pulling image %q", dockerVolumeWorkspaceImage)
		}
	}

	progress(1)
	return nil
}

func (svc *Service) BuildTasks(ctx context.Context, repos []*graphql.Repository, spec *CampaignSpec) ([]*Task, error) {
	workspaceConfigs := []WorkspaceConfiguration{}
	for _, conf := range spec.Workspaces {
		g, err := glob.Compile(conf.In)
		if err != nil {
			return nil, err
		}
		conf.glob = g
		workspaceConfigs = append(workspaceConfigs, conf)
	}

	// rootWorkspace contains all the repositories that didn't match a
	// `workspaces` configuration.
	rootWorkspace := map[*graphql.Repository]struct{}{}
	// reposByWorkspaceConfig maps workspace config to repositories in which
	// the workspace config should be used.
	reposByWorkspaceConfig := make(map[int][]*graphql.Repository, len(workspaceConfigs))

	for _, repo := range repos {
		matched := false

		for i, conf := range workspaceConfigs {
			if !conf.glob.Match(repo.Name) {
				continue
			}

			if matched {
				return nil, fmt.Errorf("repository %s matches multiple workspaces.in globs in campaign spec. glob: %q", repo.Name, conf.In)
			}

			if rs, ok := reposByWorkspaceConfig[i]; ok {
				reposByWorkspaceConfig[i] = append(rs, repo)
			} else {
				reposByWorkspaceConfig[i] = []*graphql.Repository{repo}
			}
			matched = true
		}

		if !matched {
			rootWorkspace[repo] = struct{}{}
		}
	}

	var tasks []*Task

	for configIndex, repos := range reposByWorkspaceConfig {
		workspaceConfig := workspaceConfigs[configIndex]
		repoDirs, err := svc.FindDirectoriesInRepos(ctx, workspaceConfig.RootAtLocationOf, repos...)
		if err != nil {
			return nil, err
		}

		for repo, dirs := range repoDirs {
			for _, d := range dirs {
				// Directory is root.
				if d == "." {
					// This shouldn't happen, but sanity check:
					if _, ok := rootWorkspace[repo]; ok {
						continue
					} else {
						d = ""
					}
				}

				tasks = append(tasks, &Task{
					Repository:         repo,
					Path:               d,
					Steps:              spec.Steps,
					TransformChanges:   spec.TransformChanges,
					Template:           spec.ChangesetTemplate,
					OnlyFetchWorkspace: workspaceConfig.OnlyFetchWorkspace,
				})
			}
		}
	}

	for r := range rootWorkspace {
		tasks = append(tasks, &Task{
			Repository:       r,
			Path:             "",
			Steps:            spec.Steps,
			TransformChanges: spec.TransformChanges,
			Template:         spec.ChangesetTemplate,
		})
	}

	return tasks, nil
}

func (svc *Service) ExecuteCampaignSpec(ctx context.Context, opts ExecutorOpts, tasks []*Task, spec *CampaignSpec, progress func([]*TaskStatus), skipErrors bool) ([]*ChangesetSpec, []string, error) {
	x := newExecutor(opts, svc.client, svc.features)
	for _, t := range tasks {
		x.AddTask(t)
	}

	done := make(chan struct{})
	if progress != nil {
		go func() {
			x.LockedTaskStatuses(progress)

			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					x.LockedTaskStatuses(progress)

				case <-done:
					return
				}
			}
		}()
	}

	var errs *multierror.Error

	x.Start(ctx)
	specs, err := x.Wait(ctx)
	if progress != nil {
		x.LockedTaskStatuses(progress)
		done <- struct{}{}
	}
	if err != nil {
		if skipErrors {
			errs = multierror.Append(errs, err)
		} else {
			return nil, nil, err
		}
	}

	// Add external changeset specs.
	for _, ic := range spec.ImportChangesets {
		repo, err := svc.resolveRepositoryName(ctx, ic.Repository)
		if err != nil {
			wrapped := errors.Wrapf(err, "resolving repository name %q", ic.Repository)
			if skipErrors {
				errs = multierror.Append(errs, wrapped)
				continue
			} else {
				return nil, nil, wrapped
			}
		}

		for _, id := range ic.ExternalIDs {
			var sid string

			switch tid := id.(type) {
			case string:
				sid = tid
			case int, int8, int16, int32, int64:
				sid = strconv.FormatInt(reflect.ValueOf(id).Int(), 10)
			case uint, uint8, uint16, uint32, uint64:
				sid = strconv.FormatUint(reflect.ValueOf(id).Uint(), 10)
			case float32:
				sid = strconv.FormatFloat(float64(tid), 'f', -1, 32)
			case float64:
				sid = strconv.FormatFloat(tid, 'f', -1, 64)
			default:
				return nil, nil, errors.Errorf("cannot convert value of type %T into a valid external ID: expected string or int", id)
			}

			specs = append(specs, &ChangesetSpec{
				BaseRepository:    repo.ID,
				ExternalChangeset: &ExternalChangeset{sid},
			})
		}
	}

	return specs, x.LogFiles(), errs.ErrorOrNil()
}

func (svc *Service) ValidateChangesetSpecs(repos []*graphql.Repository, specs []*ChangesetSpec) error {
	repoByID := make(map[string]*graphql.Repository, len(repos))
	for _, repo := range repos {
		repoByID[repo.ID] = repo
	}

	byRepoAndBranch := make(map[string]map[string][]*ChangesetSpec)
	for _, spec := range specs {
		// We don't need to validate imported changesets, as they can
		// never have a critical branch name overlap.
		if spec.ExternalChangeset != nil {
			continue
		}
		if _, ok := byRepoAndBranch[spec.HeadRepository]; !ok {
			byRepoAndBranch[spec.HeadRepository] = make(map[string][]*ChangesetSpec)
		}

		byRepoAndBranch[spec.HeadRepository][spec.HeadRef] = append(byRepoAndBranch[spec.HeadRepository][spec.HeadRef], spec)
	}

	duplicates := make(map[*graphql.Repository]map[string]int)
	for repoID, specsByBranch := range byRepoAndBranch {
		for branch, specs := range specsByBranch {
			if len(specs) < 2 {
				continue
			}

			r := repoByID[repoID]
			if _, ok := duplicates[r]; !ok {
				duplicates[r] = make(map[string]int)
			}

			duplicates[r][branch] = len(specs)
		}
	}

	if len(duplicates) > 0 {
		return &duplicateBranchesErr{duplicates: duplicates}
	}

	return nil
}

type duplicateBranchesErr struct {
	duplicates map[*graphql.Repository]map[string]int
}

func (e *duplicateBranchesErr) Error() string {
	var out strings.Builder

	fmt.Fprintf(&out, "Multiple changeset specs have the same branch:\n\n")

	for repo, branches := range e.duplicates {
		for branch, duplicates := range branches {
			branch = strings.TrimPrefix(branch, "refs/heads/")
			fmt.Fprintf(&out, "\t* %s: %d changeset specs have the branch %q\n", repo.Name, duplicates, branch)
		}
	}

	fmt.Fprint(&out, "\nMake sure that the changesetTemplate.branch field in the campaign spec produces unique values for each changeset in a single repository and rerun this command.")

	return out.String()
}

func (svc *Service) ParseCampaignSpec(in io.Reader) (*CampaignSpec, string, error) {
	data, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, "", errors.Wrap(err, "reading campaign spec")
	}

	spec, err := ParseCampaignSpec(data, svc.features)
	if err != nil {
		return nil, "", errors.Wrap(err, "parsing campaign spec")
	}
	return spec, string(data), nil
}

const namespaceQuery = `
query NamespaceQuery($name: String!) {
    user(username: $name) {
        id
    }

    organization(name: $name) {
        id
    }
}
`

const usernameQuery = `
query GetCurrentUserID {
    currentUser {
        id
    }
}
`

func (svc *Service) ResolveNamespace(ctx context.Context, namespace string) (string, error) {
	if namespace == "" {
		// if no namespace is provided, default to logged in user as namespace
		var resp struct {
			Data struct {
				CurrentUser struct {
					ID string `json:"id"`
				} `json:"currentUser"`
			} `json:"data"`
		}
		if ok, err := svc.client.NewRequest(usernameQuery, nil).DoRaw(ctx, &resp); err != nil || !ok {
			return "", errors.WithMessage(err, "failed to resolve namespace: no user logged in")
		}

		if resp.Data.CurrentUser.ID == "" {
			return "", errors.New("cannot resolve current user")
		}
		return resp.Data.CurrentUser.ID, nil
	}

	var result struct {
		Data struct {
			User         *struct{ ID string }
			Organization *struct{ ID string }
		}
		Errors []interface{}
	}
	if ok, err := svc.client.NewRequest(namespaceQuery, map[string]interface{}{
		"name": namespace,
	}).DoRaw(ctx, &result); err != nil || !ok {
		return "", err
	}

	if result.Data.User != nil {
		return result.Data.User.ID, nil
	}
	if result.Data.Organization != nil {
		return result.Data.Organization.ID, nil
	}
	return "", fmt.Errorf("failed to resolve namespace %q: no user or organization found", namespace)
}

func (svc *Service) ResolveRepositories(ctx context.Context, spec *CampaignSpec) ([]*graphql.Repository, error) {
	seen := map[string]*graphql.Repository{}
	unsupported := UnsupportedRepoSet{}

	// TODO: this could be trivially parallelised in the future.
	for _, on := range spec.On {
		repos, err := svc.ResolveRepositoriesOn(ctx, &on)
		if err != nil {
			return nil, errors.Wrapf(err, "resolving %q", on.String())
		}

		for _, repo := range repos {
			if !repo.HasBranch() {
				continue
			}

			if other, ok := seen[repo.ID]; !ok {
				seen[repo.ID] = repo

				switch st := strings.ToLower(repo.ExternalRepository.ServiceType); st {
				case "github", "gitlab", "bitbucketserver":
				default:
					if !svc.allowUnsupported {
						unsupported.appendRepo(repo)
					}
				}
			} else {
				// If we've already seen this repository, we overwrite the
				// Commit/Branch fields with the latest value we have
				other.Commit = repo.Commit
				other.Branch = repo.Branch
			}
		}
	}

	final := make([]*graphql.Repository, 0, len(seen))
	for _, repo := range seen {
		if !unsupported.includes(repo) {
			final = append(final, repo)
		}
	}

	if unsupported.hasUnsupported() {
		return final, unsupported
	}

	return final, nil
}

func (svc *Service) ResolveRepositoriesOn(ctx context.Context, on *OnQueryOrRepository) ([]*graphql.Repository, error) {
	if on.RepositoriesMatchingQuery != "" {
		return svc.resolveRepositorySearch(ctx, on.RepositoriesMatchingQuery)
	} else if on.Repository != "" && on.Branch != "" {
		repo, err := svc.resolveRepositoryNameAndBranch(ctx, on.Repository, on.Branch)
		if err != nil {
			return nil, err
		}
		return []*graphql.Repository{repo}, nil
	} else if on.Repository != "" {
		repo, err := svc.resolveRepositoryName(ctx, on.Repository)
		if err != nil {
			return nil, err
		}
		return []*graphql.Repository{repo}, nil
	}

	// This shouldn't happen on any campaign spec that has passed validation,
	// but, alas, software.
	return nil, ErrMalformedOnQueryOrRepository
}

const repositoryNameQuery = `
query Repository($name: String!, $queryCommit: Boolean!, $rev: String!) {
    repository(name: $name) {
        ...repositoryFields
    }
}
` + graphql.RepositoryFieldsFragment

func (svc *Service) resolveRepositoryName(ctx context.Context, name string) (*graphql.Repository, error) {
	var result struct{ Repository *graphql.Repository }
	if ok, err := svc.client.NewRequest(repositoryNameQuery, map[string]interface{}{
		"name":        name,
		"queryCommit": false,
		"rev":         "",
	}).Do(ctx, &result); err != nil || !ok {
		return nil, err
	}
	if result.Repository == nil {
		return nil, errors.New("no repository found")
	}
	return result.Repository, nil
}

func (svc *Service) resolveRepositoryNameAndBranch(ctx context.Context, name, branch string) (*graphql.Repository, error) {
	var result struct{ Repository *graphql.Repository }
	if ok, err := svc.client.NewRequest(repositoryNameQuery, map[string]interface{}{
		"name":        name,
		"queryCommit": true,
		"rev":         branch,
	}).Do(ctx, &result); err != nil || !ok {
		return nil, err
	}
	if result.Repository == nil {
		return nil, errors.New("no repository found")
	}
	if result.Repository.Commit.OID == "" {
		return nil, fmt.Errorf("no branch matching %q found for repository %s", branch, name)
	}

	result.Repository.Branch = graphql.Branch{
		Name:   branch,
		Target: result.Repository.Commit,
	}

	return result.Repository, nil
}

// TODO: search result alerts.
const repositorySearchQuery = `
query ChangesetRepos(
    $query: String!,
	$queryCommit: Boolean!,
	$rev: String!,
) {
    search(query: $query, version: V2) {
        results {
            results {
                __typename
                ... on Repository {
                    ...repositoryFields
                }
                ... on FileMatch {
                    file { path }
                    repository {
                        ...repositoryFields
                    }
                }
            }
        }
    }
}
` + graphql.RepositoryFieldsFragment

func (svc *Service) resolveRepositorySearch(ctx context.Context, query string) ([]*graphql.Repository, error) {
	var result struct {
		Search struct {
			Results struct {
				Results []searchResult
			}
		}
	}
	if ok, err := svc.client.NewRequest(repositorySearchQuery, map[string]interface{}{
		"query":       setDefaultQueryCount(query),
		"queryCommit": false,
		"rev":         "",
	}).Do(ctx, &result); err != nil || !ok {
		return nil, err
	}

	ids := map[string]*graphql.Repository{}
	var repos []*graphql.Repository
	for _, r := range result.Search.Results.Results {
		existing, ok := ids[r.ID]
		if !ok {
			repo := r.Repository
			repos = append(repos, &repo)
			ids[r.ID] = &repo
		} else {
			for file := range r.FileMatches {
				existing.FileMatches[file] = true
			}
		}
	}
	return repos, nil
}

const findDirectoryQuery = `
query DirectoriesContainingFile($query: String!) {
    search(query: $query, version: V2) {
        results {
            results {
                __typename
                ... on FileMatch {
                    file { path }
                }
            }
        }
    }
}
`

// findDirectoriesResult maps the name of the GraphQL query to its results. The
// name is the repository's ID.
type findDirectoriesResult map[string]struct {
	Results struct{ Results []searchResult }
}

// FindDirectoriesInRepos returns a map of repositories and the locations of
// files matching the given file name in the repository.
// The locations are paths relative to the root of the directory.
// No "/" at the beginning.
// An empty path ("") represents the root directory.
func (svc *Service) FindDirectoriesInRepos(ctx context.Context, fileName string, repos ...*graphql.Repository) (map[*graphql.Repository][]string, error) {
	const batchSize = 50

	// Build up unique identifiers that are safe to use as GraphQL query aliases.
	reposByQueryID := map[string]*graphql.Repository{}
	queryIDByRepo := map[*graphql.Repository]string{}
	for i, repo := range repos {
		queryID := fmt.Sprintf("repo_%d", i)
		reposByQueryID[queryID] = repo
		queryIDByRepo[repo] = queryID
	}

	findInBatch := func(batch []*graphql.Repository, results map[*graphql.Repository][]string) error {
		const searchQueryTmpl = `%s: search(query: %q, version: V2) {
        results {
            results {
                __typename
                ... on FileMatch {
                    file { path }
                }
            }
        }
	}
`

		var a strings.Builder
		a.WriteString("query DirectoriesContainingFile() {\n")

		for _, repo := range batch {
			query := fmt.Sprintf(`file:(^|/)%s$ repo:^%s$ type:path count:99999`, regexp.QuoteMeta(fileName), regexp.QuoteMeta(repo.Name))

			a.WriteString(fmt.Sprintf(searchQueryTmpl, queryIDByRepo[repo], query))
		}

		a.WriteString("}")

		var result findDirectoriesResult
		if ok, err := svc.client.NewQuery(a.String()).Do(ctx, &result); err != nil || !ok {
			return err
		}

		for queryID, search := range result {
			repo, ok := reposByQueryID[queryID]
			if !ok {
				return fmt.Errorf("result for query %q did not match any repository", queryID)
			}

			files := map[string]struct{}{}

			for _, r := range search.Results.Results {
				for file := range r.FileMatches {
					files[file] = struct{}{}
				}
			}

			var dirs []string
			for f := range files {
				// We use path.Dir and not filepath.Dir here, because while
				// src-cli might be executed on Windows, we need the paths to
				// be Unix paths, since they will be used inside Docker
				// containers.
				dirs = append(dirs, path.Dir(f))
			}

			results[repo] = dirs
		}

		return nil
	}

	results := make(map[*graphql.Repository][]string)

	for start := 0; start < len(repos); start += batchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		end := start + batchSize
		if end > len(repos) {
			end = len(repos)
		}

		batch := repos[start:end]

		err := findInBatch(batch, results)
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

var defaultQueryCountRegex = regexp.MustCompile(`\bcount:\d+\b`)

const hardCodedCount = " count:999999"

func setDefaultQueryCount(query string) string {
	if defaultQueryCountRegex.MatchString(query) {
		return query
	}

	return query + hardCodedCount
}

type searchResult struct {
	graphql.Repository
}

func (sr *searchResult) UnmarshalJSON(data []byte) error {
	var tn struct {
		Typename string `json:"__typename"`
	}
	if err := json.Unmarshal(data, &tn); err != nil {
		return err
	}

	switch tn.Typename {
	case "FileMatch":
		var result struct {
			Repository graphql.Repository
			File       struct {
				Path string
			}
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}

		sr.Repository = result.Repository
		sr.Repository.FileMatches = map[string]bool{result.File.Path: true}
		return nil

	case "Repository":
		if err := json.Unmarshal(data, &sr.Repository); err != nil {
			return err
		}
		sr.Repository.FileMatches = map[string]bool{}
		return nil

	default:
		return errors.Errorf("unknown GraphQL type %q", tn.Typename)
	}
}
