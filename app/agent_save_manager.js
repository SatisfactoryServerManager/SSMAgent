const path = require("path");
const fs = require("fs-extra");
const rimraf = require("rimraf");
const recursive = require("recursive-readdir");
const Serializer = require("../utils/Serializer");
const FormData = require("form-data");

const Config = require("./agent_config");
const Logger = require("./agent_logger");
const Cleanup = require("./agent_cleanup");

const AgentAPI = require("./agent_api");

const VarCache = require("./agent_varcache");

class AgentSaveManager {
    constructor() {
        this._lastSaveInfo = [];
    }
    init = async () => {
        Logger.info("[SaveManager] - Initialising Save Manager ...");
        await this.UploadSaveDatas();
        await this.UploadSaveFiles();

        setInterval(async () => {
            const shouldUpload = await this.CheckShouldUpload();
            if (shouldUpload) {
                await this.UploadSaveDatas();
                await this.UploadSaveFiles();
            }
        }, 10000);
        Logger.info("[SaveManager] - Initialised Save Manager!");
    };

    DownloadSaveFile = async (data) => {
        try {
            await AgentAPI.DownloadAgentSaveFile(data.saveFile);
            await this.UploadSaveDatas();
            await this.UploadSaveFiles();
        } catch (err) {
            console.log(err);
            throw err;
        }
    };

    GetAllSaveFiles = async () => {
        const SaveFolder = VarCache.get("savedir");

        fs.ensureDirSync(SaveFolder);

        const files = await recursive(SaveFolder);
        const filesArray = [];

        for (let i = 0; i < files.length; i++) {
            const filePath = files[i];
            const fileData = await this.GetSaveFileInformation(filePath);
            filesArray.push(fileData);
        }

        filesArray.sort((a, b) =>
            a.stats.mtime > b.stats.mtime
                ? 1
                : b.stats.mtime > a.stats.mtime
                ? -1
                : 0
        );

        return filesArray;
    };

    GetSaveFileInformation = async (filePath) => {
        const returnData = {
            path: filePath,
            fileName: path.basename(filePath),
            stats: fs.statSync(filePath),
        };

        const buffer = fs.readFileSync(filePath);
        const serial = new Serializer(buffer);

        serial.seek(12);
        returnData.level = serial.readString().replaceAll("\0", "");
        returnData.startInfo = serial.readString().replaceAll("\0", "");
        returnData.sessionName = serial.readString().replaceAll("\0", "");
        serial.seek(17);

        const modsString = serial.readString().replaceAll("\0", "");

        if (modsString.IsJsonString()) {
            returnData.mods = JSON.parse(modsString).Mods;
        }

        return returnData;
    };

    CheckShouldUpload = async () => {
        try {
            const saveDatas = await this.GetAllSaveFiles();
            if (this._lastSaveInfo.length != saveDatas.length) {
                return true;
            }

            for (let i = 0; i < saveDatas.length; i++) {
                const saveData = saveDatas[i];
                const lastSave = this._lastSaveInfo.find(
                    (ls) => ls.path == saveData.path
                );

                if (lastSave == null) {
                    return true;
                }

                if (
                    lastSave.stats.mtime.getTime() !=
                    saveData.stats.mtime.getTime()
                ) {
                    return true;
                }
            }

            return false;
        } catch (err) {}
    };

    UploadSaveDatas = async () => {
        try {
            const saveDatas = await this.GetAllSaveFiles();
            await AgentAPI.remoteRequestPOST("api/agent/saves/info", {
                saveDatas,
            });

            this._lastSaveInfo = saveDatas;
        } catch (err) {
            console.log(err);
            throw err;
        }
    };

    UploadSaveFiles = async () => {
        try {
            const saveDatas = await this.GetAllSaveFiles();

            for (let i = 0; i < saveDatas.length; i++) {
                const savedata = saveDatas[i];
                await this.UploadSaveFile(
                    path.dirname(savedata.path),
                    savedata.fileName
                );
            }
        } catch (err) {
            console.log(err);
            throw err;
        }
    };

    UploadSaveFile = async (backupFilePath, backupFileName) => {
        try {
            const backupFile = path.join(backupFilePath, backupFileName);
            console.log(`Uploading Save File ${backupFile}`);

            const fileStream = fs.createReadStream(backupFile);

            const form = new FormData();
            // Pass file stream directly to form
            form.append("file", fileStream, backupFileName);

            const url = `${Config.get(
                "agent.ssmcloud.url"
            )}/api/agent/uploadsave`;

            const res = await fetch(url, {
                method: "POST",
                body: form,
                headers: {
                    "x-ssm-key": Config.get("agent.ssmcloud.apikey"),
                    ...form.getHeaders(),
                },
            });
        } catch (err) {
            console.log(err);
            throw err;
        }
    };
}

const agentSaveManager = new AgentSaveManager();
module.exports = agentSaveManager;
