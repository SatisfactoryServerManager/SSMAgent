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

        super.get(
            "agent.ssmcloud.url",
            process.env.SSM_URL || "http://localhost"
        );
        super.get("agent.ssmcloud.apikey", process.env.SSM_APIKEY || "ABC123");

        super.get("agent.sf.branch", "public");
        super.get("agent.sf.versions.installed", 0);
        super.get("agent.sf.versions.available", 0);

        super.get("agent.sf.worker_threads", 20);
    };
}

const serverConfig = new ServerConfig();

module.exports = serverConfig;
