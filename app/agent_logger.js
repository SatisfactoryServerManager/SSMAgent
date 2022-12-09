const iLogger = require("mrhid6utils").Logger;
const path = require("path");

let userDataPath = path.resolve(require("os").homedir() + "/SSMAgent");

class Logger extends iLogger {
    constructor() {
        super({
            logBaseDirectory: path.join(userDataPath, "logs"),
            logName: "SSMAgent",
        });
    }
}

const logger = new Logger();
module.exports = logger;
