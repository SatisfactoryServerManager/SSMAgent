const exec = require("child_process").exec;
const path = require("path");
const moment = require("moment");
const fs = require("fs-extra");
const recursive = require("recursive-readdir");
const es = require("event-stream");
const rimraf = require("rimraf");
const FormData = require("form-data");
const axios = require("axios");

const Config = require("./agent_config");
const logger = require("./agent_logger");
const SteamLogger = require("./agent_steamcmd").SteamLogger;
const AgentAPI = require("./agent_api");
const chmodr = require("chmodr");

class SSM_Log_Handler {
    constructor() {
        this._TotalSFLogLineCount = 0;
        this._UserCount = 0;

        this._lastLogsInfo = {};
    }

    init() {
        this.SendLogData();
        setInterval(async () => {
            try {
                await this.SendLogData();
            } catch (err) {
                console.log(err);
            }
        }, 30000);
    }

    SendLogData = async () => {
        try {
            await this.UploadLogFile(
                logger._options.logDirectory,
                "SSMAgent-combined.log"
            );
        } catch (err) {
            console.log(err);
        }

        try {
            await this.UploadLogFile(
                SteamLogger._options.logDirectory,
                "SSMSteamCMD-combined.log"
            );
        } catch (err) {
            console.log(err);
        }

        const logDir = path.join(
            Config.get("agent.sfserver"),
            "FactoryGame",
            "Saved",
            "Logs"
        );

        const logfile = path.join(logDir, "FactoryGame.log");

        if (!fs.existsSync(logfile)) {
            return;
        }

        try {
            await this.UploadLogFile(logDir, path.basename(logfile));
        } catch (err) {
            console.log(err);
        }
    };

    UploadLogFile = async (FileDirectory, FileName) => {
        let lastLogInfo;

        if (this._lastLogsInfo.hasOwnProperty(FileName)) {
            lastLogInfo = this._lastLogsInfo[`${FileName}`];
        } else {
            this._lastLogsInfo[`${FileName}`] = {
                mtime: 0,
            };
            lastLogInfo = this._lastLogsInfo[`${FileName}`];
        }

        const FilePath = path.join(FileDirectory, FileName);

        const now = new Date();
        const copiedFilePath = `${FilePath}-${now.getTime()}`;

        if (fs.existsSync(copiedFilePath)) {
            rimraf.sync(copiedFilePath);
        }

        fs.copyFileSync(FilePath, copiedFilePath);

        fs.chmodSync(copiedFilePath, "0777");

        const fileStats = fs.statSync(copiedFilePath);

        if (fileStats.mtimeMs <= lastLogInfo.mtime) {
            if (fs.existsSync(copiedFilePath)) {
                rimraf.sync(copiedFilePath);
            }
            return;
        }

        lastLogInfo.mtime = fileStats.mtimeMs;

        const fileStream = fs.createReadStream(copiedFilePath);

        const form = new FormData();
        // Pass file stream directly to form
        form.append("file", fileStream, FileName);

        const url = `${Config.get("agent.ssmcloud.url")}/api/agent/uploadlog`;

        try {
            await axios.post(url, form, {
                headers: {
                    "x-ssm-key": Config.get("agent.ssmcloud.apikey"),
                    ...form.getHeaders(),
                },
            });
        } catch (err) {
            console.log(err);
        }

        if (fs.existsSync(copiedFilePath)) {
            rimraf.sync(copiedFilePath);
        }
    };

    getLogFiles(directory) {
        return new Promise((resolve, reject) => {
            recursive(directory, [logFileFilter], (err, files) => {
                if (err) {
                    reject(err);
                    return;
                }
                resolve(files);
            });
        });
    }
}

function logFileFilter(file, stats) {
    return path.extname(file) != ".log" && stats.isDirectory() == false;
}

const ssm_log_handler = new SSM_Log_Handler();
module.exports = ssm_log_handler;
