const Config = require("./agent_config");
const Logger = require("./agent_logger");
const Cleanup = require("./agent_cleanup");

const AgentAPI = require("./agent_api");
const AgentMessageQueue = require("./agent_messagequeue");
const SteamCMD = require("./agent_steamcmd").AgentSteamCMD;
const AgentSFHandler = require("./agent_sf_handler");
const BackupManager = require("./agent_backup");
const AgentSaveManager = require("./agent_save_manager");
const AgentSaveInspector = require("./agent_save_inspector");
const AgentServerConfigManager = require("./agent_server_config");
const AgentLogHandler = require("./agent_log_handler");
const AgentModManager = require("./agent_mod_manager");

class AgentApp {
    constructor() {}

    init = async () => {
        Cleanup.addEventHandler(async () => {
            Logger.info("[AGENT_APP] [CLEANUP] - Closing Agent App...");
            try {
                await this.SendAgentOfflineRequest();
            } catch (err) {
                //console.log(err);
            }
        });

        try {
            await this.SendAgentOnlineRequest();
            await Config.SendConfigToSSMCloud();

            await AgentSaveManager.init();
            await AgentSaveInspector.init();
            await AgentServerConfigManager.init();
            AgentLogHandler.init();

            await this.setupSteamCMD();
            await AgentSFHandler.init();
            BackupManager.init();

            await AgentModManager.init();

            await AgentMessageQueue.startPollingTask();
        } catch (err) {
            //console.log(err);
        }
    };

    SendAgentOnlineRequest = async () => {
        Cleanup.addPendingFunction("App:SendAgentOnlineRequest");
        try {
            await AgentAPI.remoteRequestPOST("api/agent/activestate", {
                active: true,
            });
            Logger.info("[AGENT_APP] - Sent Online Status to SSM Cloud");
            Cleanup.completePendingFunction("App:SendAgentOnlineRequest");
        } catch (err) {
            Logger.error(
                `[AGENT_APP] - Error Sending Online Status to SSM Cloud: ${err.message} `
            );
            Cleanup.completePendingFunction("App:SendAgentOnlineRequest");
            throw err;
        }
    };

    SendAgentOfflineRequest = async () => {
        Cleanup.addPendingFunction("App:SendAgentOfflineRequest");
        try {
            await AgentAPI.remoteRequestPOST("api/agent/activestate", {
                active: false,
            });
            Cleanup.completePendingFunction("App:SendAgentOfflineRequest");
            Logger.info("[AGENT_APP] - Sent Offline Status to SSM Cloud");
        } catch (err) {
            Cleanup.completePendingFunction("App:SendAgentOfflineRequest");
            Logger.error(
                `[AGENT_APP] - Error Sending Offline Status to SSM Cloud: ${err.message} `
            );
            throw err;
        }
    };

    setupSteamCMD = async () => {
        SteamCMD.init(Config.get("agent.steamcmd"));

        if (!SteamCMD.isInstalled()) {
            Logger.info("[AGENT_APP] - Downloading SteamCMD");
            try {
                await SteamCMD.download();
                Logger.info("[AGENT_APP] - Successfully Downloaded SteamCMD");
            } catch (err) {
                Logger.error("[AGENT_APP] - Error Downloading SteamCMD");
                console.log(err);
            }
        }

        Logger.info("[AGENT_APP] - Initializing SteamCMD");
        try {
            await SteamCMD.run([], true);
            Logger.info("[AGENT_APP] - Successfully Initialized SteamCMD");
        } catch (err) {
            Logger.error("[AGENT_APP] - Error Initializing SteamCMD");
            console.log(err);
        }
    };
}

const agentApp = new AgentApp();
module.exports = agentApp;
