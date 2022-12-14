const path = require("path");
const fs = require("fs-extra");
const rimraf = require("rimraf");
const { getDataHome, getHomeFolder } = require("platform-folders");
const recursive = require("recursive-readdir");
const Serializer = require("../utils/Serializer");
const FormData = require("form-data");
const axios = require("axios");

const platform = process.platform;

const Config = require("./agent_config");
const Logger = require("./agent_logger");
const Cleanup = require("./agent_cleanup");

const AgentAPI = require("./agent_api");

class AgentSaveManager {
    constructor() {
        this._lastSaveInfo = [];
    }
    init = async () => {
        await this.UploadSaveDatas();
        await this.UploadSaveFiles();

        setInterval(async () => {
            const shouldUpload = await this.CheckShouldUpload();
            if (shouldUpload) {
                await this.UploadSaveDatas();
                await this.UploadSaveFiles();
            }
        }, 10000);
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
        let localAppdata = "";
        if (platform == "win32") {
            localAppdata = path.resolve(
                getDataHome() + "/../local/FactoryGame"
            );
        } else {
            localAppdata = path.resolve(
                getHomeFolder() + "/.config/Epic/FactoryGame"
            );
        }
        const SaveFolder = path.join(
            localAppdata,
            "Saved",
            "SaveGames",
            "server"
        );

        fs.ensureDirSync(SaveFolder);

        const files = await recursive(SaveFolder);
        const filesArray = [];

        for (let i = 0; i < files.length; i++) {
            const filePath = files[i];
            const fileData = await this.GetSaveFileInformation(filePath);
            filesArray.push(fileData);
        }

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
        returnData.mods = JSON.parse(
            serial.readString().replaceAll("\0", "")
        ).Mods;

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
            const fileStream = fs.createReadStream(backupFile);

            const form = new FormData();
            // Pass file stream directly to form
            form.append("file", fileStream, backupFileName);

            const url = `${Config.get(
                "agent.ssmcloud.url"
            )}/api/agent/uploadsave`;

            await axios.post(url, form, {
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
