import { ENVIRONMENT_CONFIG, DEFAULT_APP_SETTINGS } from './constants'
import { handleHumanMessage } from './mention-handler'
import { createCodebaseContext } from './services/codebase-context'
import { isBotEvent } from './slack/helpers'
import { app } from './slack/init'

const { PORT } = ENVIRONMENT_CONFIG

// Main function to start the bot
async function startBot() {
    // Create a context for the codebase using the default app settings
    const codebaseContext = await createCodebaseContext(DEFAULT_APP_SETTINGS.codebase, DEFAULT_APP_SETTINGS.contextType)

    // Listen for mentions in the Slack app
    app.event<'app_mention'>('app_mention', async ({ event }) => {
        // Ignore events generated by bots
        if (isBotEvent(event)) {
            return
        }

        console.log('APP_MENTION', event.text)
        // Process the mention event generated by a human user
        await handleHumanMessage(event, codebaseContext)
    })

    // Start the Slack app on the specified port
    return app.start(PORT)
}

// Start the bot and log the status
startBot()
    .then(() => console.log(`⚡️ Cody Slack-bot is running on port ${PORT}!`))
    .catch(error => {
        console.error('Error starting the bot:', error)
        process.exit(1)
    })
