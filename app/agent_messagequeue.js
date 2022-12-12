const AgentAPI = require("./agent_api");
const Logger = require("./agent_logger");

const AgentSFHandler = require("./agent_sf_handler");

class AgentMessageQueue {
    constructor() {
        this._queue = [];
    }

    startPollingTask = async () => {
        setInterval(async () => {
            if (this._processingQueue) return;

            try {
                await this.pollQueue();
            } catch (err) {
                Logger.error("[MessageQueue] - Error getting message queue.");
                //console.log(err);
            }
        }, 10000);
    };

    pollQueue = async () => {
        try {
            const data = await AgentAPI.remoteRequestGET(
                "api/agent/messagequeue"
            );
            this._queue = data.data;

            await this.processQueue();
        } catch (err) {
            throw err;
        }
    };

    processQueue = async () => {
        if (this._queue.length == 0) return;

        this._processingQueue = true;
        Logger.debug(
            `[MessageQueue] - Processing ${this._queue.length} Message Items`
        );

        for (let i = 0; i < this._queue.length; i++) {
            const queueItem = this._queue[i];
            try {
                await this.handleQueueItem(queueItem);
            } catch (err) {
                console.log(err);
            }
        }
        this._processingQueue = false;
    };

    handleQueueItem = async (item) => {
        console.log(item);
        try {
            switch (item.action) {
                case "installsfserver":
                    await AgentSFHandler.InstallSFServer();
                    break;
                case "updatesfserver":
                    await AgentSFHandler.UpdateSFServer();
                    break;
                case "startsfserver":
                    await AgentSFHandler.StartSFServer();
                    break;
                case "stopsfserver":
                    await AgentSFHandler.StopSFServer();
                    break;
                case "killsfserver":
                    await AgentSFHandler.KillSFServer();
                    break;
                default:
                    Logger.error(
                        `[MessageQueue] - Unknown Queue Action (${item.action})`
                    );
                    throw new Error(`Unknown Queue Action (${item.action})`);
            }
            item.completed = true;
            await this.UpdateMessageQueueItemCompleted(item);
        } catch (err) {
            console.log(err);
            item.completed = false;
            item.error = err.message;
            item.retries++;
            await this.UpdateMessageQueueItemCompleted(item);
        }
    };

    UpdateMessageQueueItemCompleted = async (item) => {
        try {
            await AgentAPI.remoteRequestPOST("api/agent/messagequeue", {
                item,
            });
        } catch (err) {
            throw err;
        }
    };
}

const agentMessageQueue = new AgentMessageQueue();
module.exports = agentMessageQueue;
