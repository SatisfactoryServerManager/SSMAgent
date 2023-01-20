const iLogger = require("mrhid6utils").Logger;
const path = require("path");

const VarCache = require("./agent_varcache");

class Logger extends iLogger {
    init() {
        super.init({
            logBaseDirectory: path.join(VarCache.get("homedir"), "logs"),
            logName: "SSMAgent",
        });
    }
}

const logger = new Logger();
module.exports = logger;
