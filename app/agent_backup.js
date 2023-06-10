const path = require("path");
const fs = require("fs-extra");
const archiver = require("archiver");
const platform = process.platform;
const recursive = require("recursive-readdir");
const { rimraf, rimrafSync, native, nativeSync } = require("rimraf");
const FormData = require("form-data");

const fetch = require("isomorphic-fetch");

const { getDataHome, getHomeFolder } = require("platform-folders");

const Config = require("./agent_config");
const Cleanup = require("./agent_cleanup");
const Logger = require("./agent_logger");
const SteamLogger = require("./agent_steamcmd").SteamLogger;

const AgentAPI = require("./agent_api");

class BackupManager {
    init() {
        Logger.info("[BACKUP_MANAGER] [INIT] - Backup Manager Initialising.. ");

        let PlatformFolder = "";
        if (platform == "win32") {
            PlatformFolder = "WindowsServer";
        } else {
            PlatformFolder = "LinuxServer";
        }

        this.GameConfigDir = path.join(
            Config.get("agent.sfserver"),
            "FactoryGame",
            "Saved",
            "Config",
            PlatformFolder
        );

        this.startBackupTimer();

        Logger.info("[BACKUP_MANAGER] [INIT] - Backup Manager Initialised");
    }

    startBackupTimer() {
        setInterval(() => {
            const date = new Date();
            if (Config.get("agent.backup.nextbackup") < date.getTime()) {
                this.ExecBackupTask().then(() => {
                    return this.CleanupBackupFiles();
                });
            }

            this.CleanupBackupFiles();
        }, 5000);
    }

    ExecBackupTask() {
        return new Promise((resolve, reject) => {
            const date = new Date();
            const date_Year = date.getFullYear();
            const date_Month = (date.getMonth() + 1).pad(2);
            const date_Day = date.getDate().pad(2);
            const date_Hour = date.getHours().pad(2);
            const date_Min = date.getMinutes().pad(2);
            const backupFile = `${date_Year}${date_Month}${date_Day}_${date_Hour}${date_Min}_Backup.zip`;
            const backupFilePath = path.join(
                Config.get("agent.backupdir"),
                backupFile
            );

            const interval =
                parseInt(Config.get("agent.backup.interval")) * 60 * 60 * 1000;

            const NextBackupTime = new Date(date.getTime() + interval);

            Logger.info("[BACKUP_MANAGER] - Starting Backup Task..");
            Cleanup.increaseCounter(1);

            fs.ensureDirSync(Config.get("agent.backupdir"));

            var outputStream = fs.createWriteStream(backupFilePath);
            var archive = archiver("zip");

            outputStream.on("close", async () => {
                Logger.info("[BACKUP_MANAGER] - Backup Task Finished!");
                Cleanup.decreaseCounter(1);
                Config.set("agent.backup.nextbackup", NextBackupTime.getTime());
                await Config.SendConfigToSSMCloud();
                await this.UploadBackupFile(
                    Config.get("agent.backupdir"),
                    backupFile
                );
                resolve();
            });

            archive.on("error", function (err) {
                Cleanup.decreaseCounter(1);
                reject(err);
            });

            archive.pipe(outputStream);

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
            const LogFolder = path.join(
                Config.get("agent.sfserver"),
                "FactoryGame",
                "Saved",
                "Logs"
            );

            // append files from a sub-directory, putting its contents at the root of archive
            archive.directory(SaveFolder, "Saves");
            archive.directory(LogFolder, "Logs/Server");
            archive.directory(Logger._options.logDirectory, "Logs/SSM");
            archive.directory(SteamLogger._options.logDirectory, "Logs/Steam");
            archive.directory(this.GameConfigDir, "Configs/Game");
            archive.file(Config._options.configFilePath, {
                name: "Configs/SSM/SSM.json",
            });

            //archive.directory(Config.get("ssm.manifestdir"), "Manifest");

            archive.finalize();
        });
    }

    CleanupBackupFiles() {
        return new Promise((resolve, reject) => {
            recursive(
                Config.get("agent.backupdir"),
                [BackupFileFilter],
                (err, files) => {
                    if (err) {
                        reject(err);
                        return;
                    }
                    const sortedFiles = files.sort().reverse();
                    const filesToRemove = [];

                    for (let i = 0; i < sortedFiles.length; i++) {
                        const file = sortedFiles[i];
                        if (i >= Config.get("agent.backup.keep")) {
                            filesToRemove.push(file);
                        }
                    }
                    const removePromises = [];
                    for (let i = 0; i < filesToRemove.length; i++) {
                        const file = filesToRemove[i];
                        removePromises.push(this.RemoveBackupFile(file));
                    }
                    Promise.all(removePromises).then(() => {
                        resolve();
                    });
                }
            );
        });
    }

    RemoveBackupFile = async (file) => {
        try {
            await rimraf(file, ["unlink"]);
        } catch (err) {
            Logger.error("[BACKUP_MANAGER] - Remove Backup Error");
            throw err;
        }
    };

    UploadBackupFile = async (backupFilePath, backupFileName) => {
        try {
            const backupFile = path.join(backupFilePath, backupFileName);

            const fileStream = fs.createReadStream(backupFile);

            const form = new FormData();
            // Pass file stream directly to form
            form.append("file", fileStream, backupFileName);

            const url = `${Config.get(
                "agent.ssmcloud.url"
            )}/api/agent/uploadbackup`;

            const res = await fetch(url, {
                method: "POST",
                body: form,
                headers: {
                    "x-ssm-key": Config.get("agent.ssmcloud.apikey"),
                    ...form.getHeaders(),
                },
            });
        } catch (err) {}
    };
}

function BackupFileFilter(file, stats) {
    return path.extname(file) != ".zip" && stats.isDirectory() == false;
}

const backupManager = new BackupManager();
module.exports = backupManager;
