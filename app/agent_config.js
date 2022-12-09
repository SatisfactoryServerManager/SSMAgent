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

        super.set("agent.tempdir", path.join(userDataPath, "temp"));
        fs.ensureDirSync(super.get("agent.tempdir"));

        super.get(
            "agent.ssmcloud.url",
            process.env.SSM_URL || "http://localhost"
        );
        super.get("agent.ssmcloud.apikey", process.env.SSM_APIKEY || "ABC123");
    };
}

const serverConfig = new ServerConfig();

module.exports = serverConfig;
