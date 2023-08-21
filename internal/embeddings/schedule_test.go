package embeddings

import (
	"context"
	"testing"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/background/repo"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestScheduleRepositoriesForEmbedding(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(logger, t))
	repoStore := db.Repos()

	originalConf := setSiteConfig(true, "sourcegraph", "model", "token")
	t.Cleanup(func() { revertSiteConfig(originalConf) })

	createdRepo := &types.Repo{Name: "github.com/sourcegraph/sourcegraph", URI: "github.com/sourcegraph/sourcegraph", ExternalRepo: api.ExternalRepoSpec{}}
	err := repoStore.Create(ctx, createdRepo)
	require.NoError(t, err)

	// Create a repo embedding job.
	store := repo.NewRepoEmbeddingJobsStore(db)
	_, err = store.CreateRepoEmbeddingJob(ctx, createdRepo.ID, "coffee", "sourcegraphProvider/model")
	require.NoError(t, err)

	gitserverClient := gitserver.NewMockClient()
	gitserverClient.GetDefaultBranchFunc.SetDefaultReturn("main", "coffee", nil)

	// By default, we shouldn't schedule a new job for the same revision
	repoNames := []api.RepoName{"github.com/sourcegraph/sourcegraph"}
	err = ScheduleRepositoriesForEmbedding(ctx, repoNames, false, db, store, gitserverClient)
	require.NoError(t, err)
	count, err := store.CountRepoEmbeddingJobs(ctx, repo.ListOpts{})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// With the 'force' argument, a new job will be scheduled anyways
	err = ScheduleRepositoriesForEmbedding(ctx, repoNames, true, db, store, gitserverClient)
	require.NoError(t, err)
	count, err = store.CountRepoEmbeddingJobs(ctx, repo.ListOpts{})
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestScheduleRepositoriesForEmbeddingInvalidConfiguration(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(logger, t))
	repoStore := db.Repos()
	errStr := "embeddings not configured or disabled"

	createdRepo := &types.Repo{Name: "github.com/sourcegraph/sourcegraph", URI: "github.com/sourcegraph/sourcegraph", ExternalRepo: api.ExternalRepoSpec{}}
	err := repoStore.Create(ctx, createdRepo)
	require.NoError(t, err)

	// Create a repo embedding job.
	store := repo.NewRepoEmbeddingJobsStore(db)

	gitserverClient := gitserver.NewMockClient()
	gitserverClient.GetDefaultBranchFunc.SetDefaultReturn("main", "coffee", nil)

	// By default, we shouldn't schedule a new job for the same revision
	repoNames := []api.RepoName{"github.com/sourcegraph/sourcegraph"}

	originalConf := setSiteConfig(false, "sourcegraph", "model", "token")
	t.Cleanup(func() { revertSiteConfig(originalConf) })

	err = ScheduleRepositoriesForEmbedding(ctx, repoNames, false, db, store, gitserverClient)
	require.EqualError(t, err, errStr)
	count, err := store.CountRepoEmbeddingJobs(ctx, repo.ListOpts{})
	require.NoError(t, err)
	require.Equal(t, 0, count)

	_ = setSiteConfig(true, "unknownProvider", "model", "token")
	err = ScheduleRepositoriesForEmbedding(ctx, repoNames, false, db, store, gitserverClient)
	require.EqualError(t, err, errStr)
	count, err = store.CountRepoEmbeddingJobs(ctx, repo.ListOpts{})
	require.NoError(t, err)
	require.Equal(t, 0, count)

	_ = setSiteConfig(true, "sourcegraph", "model", "")
	err = ScheduleRepositoriesForEmbedding(ctx, repoNames, false, db, store, gitserverClient)
	require.EqualError(t, err, errStr)
	count, err = store.CountRepoEmbeddingJobs(ctx, repo.ListOpts{})
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Resolve site configuration to enable scheduling of jobs
	_ = setSiteConfig(true, "sourcegraph", "model", "token")
	err = ScheduleRepositoriesForEmbedding(ctx, repoNames, false, db, store, gitserverClient)
	require.NoError(t, err)
	count, err = store.CountRepoEmbeddingJobs(ctx, repo.ListOpts{})
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestScheduleRepositoriesForEmbeddingRepoNotFound(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(logger, t))
	repoStore := db.Repos()

	originalConf := setSiteConfig(true, "sourcegraph", "model", "token")
	t.Cleanup(func() { revertSiteConfig(originalConf) })

	createdRepo0 := &types.Repo{Name: "github.com/sourcegraph/sourcegraph", URI: "github.com/sourcegraph/sourcegraph", ExternalRepo: api.ExternalRepoSpec{}}
	err := repoStore.Create(ctx, createdRepo0)
	require.NoError(t, err)

	// Create a repo embedding job.
	store := repo.NewRepoEmbeddingJobsStore(db)

	gitserverClient := gitserver.NewMockClient()
	gitserverClient.GetDefaultBranchFunc.PushReturn("main", "sgrevision", nil)

	repoNames := []api.RepoName{"github.com/repo/notfound", "github.com/sourcegraph/sourcegraph"}
	err = ScheduleRepositoriesForEmbedding(ctx, repoNames, false, db, store, gitserverClient)
	require.NoError(t, err)
	count, err := store.CountRepoEmbeddingJobs(ctx, repo.ListOpts{})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	pattern := "github.com/sourcegraph/sourcegraph"
	first := 10
	jobs, err := store.ListRepoEmbeddingJobs(ctx, repo.ListOpts{PaginationArgs: &database.PaginationArgs{First: &first, OrderBy: database.OrderBy{{Field: "id"}}, Ascending: true}, Query: &pattern})
	require.NoError(t, err)
	require.Equal(t, "queued", jobs[0].State)
}

func TestScheduleRepositoriesForEmbeddingInvalidDefaultBranch(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(logger, t))
	repoStore := db.Repos()

	originalConf := setSiteConfig(true, "sourcegraph", "model", "token")
	t.Cleanup(func() { revertSiteConfig(originalConf) })

	createdRepo0 := &types.Repo{Name: "github.com/sourcegraph/sourcegraph", URI: "github.com/sourcegraph/sourcegraph", ExternalRepo: api.ExternalRepoSpec{}}
	err := repoStore.Create(ctx, createdRepo0)
	require.NoError(t, err)

	// Create a repo embedding job.
	store := repo.NewRepoEmbeddingJobsStore(db)

	gitserverClient := gitserver.NewMockClient()
	gitserverClient.GetDefaultBranchFunc.PushReturn("", "sgrevision", nil)

	repoNames := []api.RepoName{"github.com/sourcegraph/sourcegraph"}
	err = ScheduleRepositoriesForEmbedding(ctx, repoNames, false, db, store, gitserverClient)
	require.NoError(t, err)
	count, err := store.CountRepoEmbeddingJobs(ctx, repo.ListOpts{})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	pattern := "github.com/sourcegraph/sourcegraph"
	first := 10
	jobs, err := store.ListRepoEmbeddingJobs(ctx, repo.ListOpts{PaginationArgs: &database.PaginationArgs{First: &first, OrderBy: database.OrderBy{{Field: "id"}}, Ascending: true}, Query: &pattern})
	require.NoError(t, err)
	require.Equal(t, "queued", jobs[0].State)
}

func TestScheduleRepositoriesForEmbeddingFailed(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(logger, t))
	repoStore := db.Repos()

	originalConf := setSiteConfig(true, "sourcegraph", "model", "token")
	t.Cleanup(func() { revertSiteConfig(originalConf) })

	createdRepo0 := &types.Repo{Name: "github.com/sourcegraph/sourcegraph", URI: "github.com/sourcegraph/sourcegraph", ExternalRepo: api.ExternalRepoSpec{}}
	err := repoStore.Create(ctx, createdRepo0)
	require.NoError(t, err)

	createdRepo1 := &types.Repo{Name: "github.com/sourcegraph/zoekt", URI: "github.com/sourcegraph/zoekt", ExternalRepo: api.ExternalRepoSpec{}}
	err = repoStore.Create(ctx, createdRepo1)
	require.NoError(t, err)

	// Create a repo embedding job.
	store := repo.NewRepoEmbeddingJobsStore(db)

	gitserverClient := gitserver.NewMockClient()
	gitserverClient.GetDefaultBranchFunc.PushReturn("", "sgrevision", nil)
	gitserverClient.GetDefaultBranchFunc.PushReturn("main", "zoektrevision", nil)

	repoNames := []api.RepoName{"github.com/sourcegraph/sourcegraph", "github.com/sourcegraph/zoekt"}
	err = ScheduleRepositoriesForEmbedding(ctx, repoNames, false, db, store, gitserverClient)
	require.NoError(t, err)
	count, err := store.CountRepoEmbeddingJobs(ctx, repo.ListOpts{})
	require.NoError(t, err)
	require.Equal(t, 2, count)

	pattern := "github.com/sourcegraph/sourcegraph"
	first := 10
	jobs, err := store.ListRepoEmbeddingJobs(ctx, repo.ListOpts{PaginationArgs: &database.PaginationArgs{First: &first, OrderBy: database.OrderBy{{Field: "id"}}, Ascending: true}, Query: &pattern})
	require.NoError(t, err)
	require.Equal(t, "queued", jobs[0].State)

	sgJobID := jobs[0].ID

	pattern = "github.com/sourcegraph/zoekt"
	jobs, err = store.ListRepoEmbeddingJobs(ctx, repo.ListOpts{PaginationArgs: &database.PaginationArgs{First: &first, OrderBy: database.OrderBy{{Field: "id"}}, Ascending: true}, Query: &pattern})
	require.NoError(t, err)
	require.Equal(t, "queued", jobs[0].State)

	zoektJobID := jobs[0].ID

	// Set jobs to expected completion states, with empty repo resulting in failed
	setJobState(t, ctx, store, sgJobID, "failed")
	setJobState(t, ctx, store, zoektJobID, "failed")

	// Reschedule
	gitserverClient.GetDefaultBranchFunc.PushReturn("", "sgrevision", nil)
	gitserverClient.GetDefaultBranchFunc.PushReturn("main", "zoektrevision", nil)

	err = ScheduleRepositoriesForEmbedding(ctx, repoNames, false, db, store, gitserverClient)
	require.NoError(t, err)
	count, err = store.CountRepoEmbeddingJobs(ctx, repo.ListOpts{})
	require.NoError(t, err)
	// failed job is rescheduled unless revision is empty
	require.Equal(t, 3, count)

	// repo with previous failure due to empty revision is rescheduled when repo is valid (error is nil and ref is non-empty)
	gitserverClient.GetDefaultBranchFunc.PushReturn("main", "sgrevision", nil)
	gitserverClient.GetDefaultBranchFunc.PushReturn("main", "zoektrevision", nil)

	err = ScheduleRepositoriesForEmbedding(ctx, repoNames, false, db, store, gitserverClient)
	require.NoError(t, err)
	count, err = store.CountRepoEmbeddingJobs(ctx, repo.ListOpts{})
	require.NoError(t, err)
	// failed job is rescheduled for sourcegraph once repo is valid
	require.Equal(t, 4, count)
}

func setJobState(t *testing.T, ctx context.Context, store repo.RepoEmbeddingJobsStore, jobID int, state string) {
	t.Helper()
	err := store.Exec(ctx, sqlf.Sprintf("UPDATE repo_embedding_jobs SET state = %s, finished_at = now() WHERE id = %s", state, jobID))
	if err != nil {
		t.Fatalf("failed to set repo embedding job state: %s", err)
	}
}

// set site config to enable embeddings with a provider and model.
// return the original configuration.
func setSiteConfig(codyEnabled bool, provider, model, token string) *scheduleTestConfig {
	originalEnabled := conf.Get().CodyEnabled
	originalEmbeddings := conf.Get().Embeddings

	conf.Get().CodyEnabled = &codyEnabled
	conf.Get().Embeddings = &schema.Embeddings{
		Provider:    provider,
		Model:       model,
		AccessToken: token,
	}
	return &scheduleTestConfig{
		codyEnabled:      originalEnabled,
		embeddingsConfig: originalEmbeddings,
	}
}

func revertSiteConfig(c *scheduleTestConfig) {
	conf.Get().CodyEnabled = c.codyEnabled
	conf.Get().Embeddings = c.embeddingsConfig
}

type scheduleTestConfig struct {
	codyEnabled      *bool
	embeddingsConfig *schema.Embeddings
}
