const fs = require("fs-extra");
const path = require("path");
const CryptoJS = require("crypto-js");

const semver = require("semver");

const mrhid6utils = require("mrhid6utils");
const iConfig = mrhid6utils.Config;
const platform = process.platform;

let userDataPath = path.resolve(require("os").homedir() + "/SSMAgent");

if (fs.pathExistsSync(userDataPath) == false) {
    fs.mkdirSync(userDataPath);
}

class ServerConfig extends iConfig {
    constructor() {
        super({
            configName: "SSMAgent",
            createConfig: true,
            useExactPath: true,
            configBaseDirectory: path.join(userDataPath, "configs"),
        });
    }

    setDefaultValues = async () => {
        var pjson = require("../package.json");
        super.set("agent.version", pjson.version);

        super.set("agent.steamcmd", path.join(userDataPath, "steamcmd"));
        fs.ensureDirSync(super.get("agent.steamcmd"));

        super.set("agent.sfserver", path.join(userDataPath, "sfserver"));
        fs.ensureDirSync(super.get("agent.sfserver"));

        super.set("agent.tempdir", path.join(userDataPath, "temp"));
        fs.ensureDirSync(super.get("agent.tempdir"));

        super.set("agent.backupdir", path.join(userDataPath, "backup"));
        fs.ensureDirSync(super.get("agent.backupdir"));

        super.get(
            "agent.ssmcloud.url",
            process.env.SSM_URL || "http://localhost"
        );

        if (super.get("agent.ssmcloud.apikey") != process.env.SSM_APIKEY) {
            if (
                process.env.SSM_APIKEY != null &&
                process.env.SSM_APIKEY != ""
            ) {
                super.set("agent.ssmcloud.apikey", process.env.SSM_APIKEY);
            }
        }

        super.get("agent.sf.branch", "public");
        super.get("agent.sf.versions.installed", 0);
        super.get("agent.sf.versions.available", 0);

        super.get("agent.sf.worker_threads", 20);
        super.get("agent.sf.max_players", 4);
        super.get("agent.sf.checkForUpdatesOnStart", true);

        super.get("agent.backup.interval", 1);
        super.get("agent.backup.keep", 24);
        super.get("agent.backup.nextbackup", 0);
    };

    SendConfigToSSMCloud = async () => {
        const AgentAPI = require("./agent_api");
        const payload = {
            config: {
                version: super.get("agent.version"),
                workerThreads: super.get("agent.sf.worker_threads"),
                sfVersions: super.get("agent.sf.versions"),
                sfBranch: super.get("agent.sf.branch"),
                maxPlayers: super.get("agent.sf.max_players"),
                checkForUpdatesOnStart: super.get(
                    "agent.sf.checkForUpdatesOnStart"
                ),
                backup: super.get("agent.backup"),
            },
        };

        try {
            await AgentAPI.remoteRequestPOST("api/agent/configData", payload);
        } catch (err) {
            console.log(err);
            throw err;
        }
    };

    UpdateSettings = async (data) => {
        if (data.inp_maxplayers) {
            const serverConfig = require("./agent_server_config");

            super.set("agent.sf.max_players", parseInt(data.inp_maxplayers));
            serverConfig
                .getGameConfig()
                .set(
                    "/Script/Engine.GameSession.MaxPlayers",
                    parseInt(data.inp_maxplayers)
                );
        }

        if (data.inp_updatesfonstart) {
            super.set(
                "agent.sf.checkForUpdatesOnStart",
                data.inp_updatesfonstart == "on" ? true : false
            );
        }

        if (data.inp_workerthreads) {
            super.set(
                "agent.sf.worker_threads",
                parseInt(data.inp_workerthreads)
            );
        }

        if (data.inp_sfbranch) {
            super.set(
                "agent.sf.branch",
                data.inp_sfbranch == "on" ? "experimental" : "public"
            );
        }

        if (data.inp_backupinterval) {
            super.set(
                "agent.backup.interval",
                parseInt(data.inp_backupinterval)
            );
        }

        if (data.inp_backupkeep) {
            super.set("agent.backup.keep", parseInt(data.inp_backupkeep));
        }

        await this.SendConfigToSSMCloud();
    };
}

const serverConfig = new ServerConfig();

module.exports = serverConfig;
