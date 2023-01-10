const iLogger = require("mrhid6utils").Logger;
const path = require("path");

const yargs = require("yargs");

const argv = yargs.parsed.argv;

let userDataPath = path.resolve(require("os").homedir() + "/SSMAgent");

if (argv.standalone) {
    userDataPath = path.join(userDataPath, argv.name);
}

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
