const fs = require("fs-extra");
const path = require("path");
const CryptoJS = require("crypto-js");
const yargs = require("yargs");

const mrhid6utils = require("mrhid6utils");
const iConfig = mrhid6utils.Config;
const platform = process.platform;

const VarCache = require("./agent_varcache");

const argv = yargs
    .option("standalone", {
        alias: "s",
        description: "Run SSM Agent in standalone mode",
        type: "boolean",
    })
    .option("name", {
        alias: "n",
        description: "(Standalone) - Name of the standalone instance",
        type: "string",
    })
    .option("portoffset", {
        alias: "p",
        description: "(Standalone) - SF Port Offset of the standalone instance",
        type: "number",
    })
    .option("ssmurl", {
        alias: "u",
        description: "(Standalone) - SSM Url",
        type: "string",
    })
    .option("ssmapikey", {
        alias: "a",
        description: "(Standalone) - SSM API Key",
        type: "string",
    })
    .help()
    .alias("help", "h").argv;

class ServerConfig extends iConfig {
    init() {
        if (argv.standalone) {
            if (argv.name == null) {
                throw new Error(
                    "Can't start in standalone without name parameter"
                );
            }

            if (argv.portoffset == null) {
                throw new Error(
                    "Can't start in standalone without port offset parameter"
                );
            }

            if (argv.ssmurl == null) {
                throw new Error(
                    "Can't start in standalone without ssm url parameter"
                );
            }

            if (argv.ssmapikey == null) {
                throw new Error(
                    "Can't start in standalone without ssm api key parameter"
                );
            }

            VarCache.set(
                "homedir",
                path.join(VarCache.get("homedir"), argv.name)
            );
        }

        super.init({
            configName: "SSMAgent",
            createConfig: true,
            useExactPath: true,
            configBaseDirectory: path.join(VarCache.get("homedir"), "configs"),
        });
    }

    setDefaultValues = async () => {
        super.set("agent.standalone", argv.standalone || false);

        super.set("agent.homedir", VarCache.get("homedir"));

        var pjson = require("../package.json");
        super.set("agent.version", pjson.version);

        super.set(
            "agent.steamcmd",
            path.join(VarCache.get("homedir"), "steamcmd")
        );
        fs.ensureDirSync(super.get("agent.steamcmd"));

        super.set(
            "agent.sfserver",
            path.join(VarCache.get("homedir"), "sfserver")
        );
        fs.ensureDirSync(super.get("agent.sfserver"));

        super.set("agent.tempdir", path.join(VarCache.get("homedir"), "temp"));
        fs.ensureDirSync(super.get("agent.tempdir"));

        super.set(
            "agent.backupdir",
            path.join(VarCache.get("homedir"), "backup")
        );
        fs.ensureDirSync(super.get("agent.backupdir"));

        if (super.get("agent.standalone")) {
            super.set("agent.sf.portoffset", argv.portoffset);
            super.set("agent.ssmcloud.url", argv.ssmurl);
            super.set("agent.ssmcloud.apikey", argv.ssmapikey);
        } else {
            super.set("agent.sf.portoffset", 0);
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
        } catch (err) {}
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
