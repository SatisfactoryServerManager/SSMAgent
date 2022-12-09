const Config = require("./agent_config");
const Logger = require("./agent_logger");
const Cleanup = require("./agent_cleanup");

const AgentAPI = require("./agent_api");

class AgentApp {
    constructor() {}

    init = async () => {
        Cleanup.addEventHandler(async () => {
            Logger.info("[AGENT_APP] [CLEANUP] - Closing Agent App...");
            try {
                Cleanup.increaseCounter(1);
                await this.SendAgentOfflineRequest();
                Cleanup.decreaseCounter(1);
            } catch (err) {}
        });
        try {
            await this.SendAgentOnlineRequest();
        } catch (err) {
            console.log(err);
        }
    };

    SendAgentOnlineRequest = async () => {
        try {
            await AgentAPI.remoteRequestPOST("api/agent/activestate", {
                active: true,
            });
            Logger.info("[AGENT_APP] - Sent Online Status to SSM Cloud");
        } catch (err) {
            throw err;
        }
    };

    SendAgentOfflineRequest = async () => {
        try {
            await AgentAPI.remoteRequestPOST("api/agent/activestate", {
                active: false,
            });
            Logger.info("[AGENT_APP] - Sent Offline Status to SSM Cloud");
        } catch (err) {
            throw err;
        }
    };
}

const agentApp = new AgentApp();
module.exports = agentApp;
