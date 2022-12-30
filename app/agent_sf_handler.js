const path = require("path");
const fs = require("fs-extra");
const rimraf = require("rimraf");
const si = require("systeminformation");
const childProcess = require("child_process");

const Config = require("./agent_config");
const Logger = require("./agent_logger");
const Cleanup = require("./agent_cleanup");

const AgentAPI = require("./agent_api");

const SteamCMD = require("./agent_steamcmd").AgentSteamCMD;

const {
    SteamCMDNotInstalled,
    SFFailedInstall,
    SFActionFailedRunning,
    SteamCMDAlreadyInstalled,
} = require("../errors/error_steamcmd");

class AgentSFHandler {
    constructor() {
        this._running = false;
        this._processIds = {
            pid1: -1,
            pid2: -1,
        };
    }

    init = async () => {
        await this.getVersionFromSteam();
        await this.UpdateAgentInstalledState();
        await this.UpdateAgentRunningState();

        if (Config.get("agent.sf.checkForUpdatesOnStart")) {
            await this.UpdateSFServer();
        }

        this.StartPollingSFProcess();
    };

    getVersionFromSteam = async () => {
        const prevVersion = Config.get("agent.sf.versions.available");

        const VersionData = await SteamCMD.getAppInfo(1690800);
        const ServerVersion =
            VersionData.depots.branches[`${Config.get("agent.sf.branch")}`]
                .buildid;
        Config.set("agent.sf.versions.available", parseInt(ServerVersion));

        if (prevVersion != Config.get("agent.sf.versions.available")) {
            await Config.SendConfigToSSMCloud();
        }
    };

    StartPollingSFProcess() {
        setInterval(async () => {
            await this.pollSFProcess();
        }, 5000);
    }

    pollSFProcess = async () => {
        const prevRunning = this._running;
        const processList = await si.processes();

        let ExeName = "";
        let SubExeName = "";

        if (process.platform == "win32") {
            ExeName = "FactoryServer.exe";
        } else {
            ExeName = "FactoryServer.sh";
        }

        if (process.platform == "win32") {
            SubExeName = "UE4Server-Win64-Shipping.exe";
        } else {
            SubExeName = "UE4Server-Linux-Shipping";
        }

        let process1 = processList.list.find(
            (el) => el.params.includes(ExeName) || el.command.includes(ExeName)
        );
        let process2 = processList.list.find((el) => el.name == SubExeName);

        if (process1 == null || process2 == null) {
            this._processIds.pid1 = -1;
            this._processIds.pid2 = -1;
            this._cpu = 0;
            this._mem = 0;
            this._running = false;
        } else {
            this._processIds.pid1 = process1.pid;
            this._processIds.pid2 = process2.pid;
            this._cpu = process2.cpu;
            this._mem = process2.mem;
            this._running = true;
        }
        if (prevRunning != this._running) await this.UpdateAgentRunningState();

        await this.UpdateAgentUsage(this._cpu, this._mem);
    };

    isInstalled = async () => {
        let ExeFileName = "FactoryServer.sh";

        if (process.platform == "win32") {
            ExeFileName = "FactoryServer.exe";
        }

        const SFServerExe = path.join(
            Config.get("agent.sfserver"),
            ExeFileName
        );

        if (fs.existsSync(SFServerExe)) return true;
        else return false;
    };

    isServerRunning = async () => {
        return this._running;
    };

    UpdateAgentInstalledState = async () => {
        const installed = await this.isInstalled();

        try {
            await AgentAPI.remoteRequestPOST("api/agent/installedstate", {
                installed,
            });
        } catch (err) {
            console.log(err);
        }
    };

    UpdateAgentRunningState = async () => {
        const running = await this.isServerRunning();

        try {
            await AgentAPI.remoteRequestPOST("api/agent/runningstate", {
                running,
            });
        } catch (err) {
            console.log(err);
        }
    };

    UpdateAgentUsage = async (cpu, mem) => {
        try {
            await AgentAPI.remoteRequestPOST("api/agent/cpumem", {
                cpu,
                mem,
            });
        } catch (err) {
            console.log(err);
        }
    };

    InstallSFServer = async () => {
        const serverRunning = await this.isServerRunning();

        if (serverRunning) {
            throw new SFActionFailedRunning();
        }

        const installed = await this.isInstalled();

        try {
            if (!installed) {
                await this._InstallSFServer();
            } else {
                await this._RemoveSFServer();
                await this._InstallSFServer();
            }
        } catch (err) {
            console.log(err);
        }
    };

    UpdateSFServer = async () => {
        const serverRunning = await this.isServerRunning();

        if (serverRunning) {
            throw new SFActionFailedRunning();
        }

        await this.getVersionFromSteam();

        if (
            Config.get("agent.sf.versions.installed") <
            Config.get("agent.sf.versions.available")
        ) {
            Logger.info(
                `[SF_Handler] - Updating SF Dedicated Server to ${Config.get(
                    "agent.sf.versions.available"
                )}`
            );
            try {
                await this.InstallSFServer();
            } catch (err) {
                throw err;
            }
        }
    };

    _InstallSFServer = async () => {
        await this.getVersionFromSteam();

        Logger.info("[SF_Handler] - Installing SF Dedicated Server ...");

        const installPath = `${path.resolve(Config.get("agent.sfserver"))}`;

        if (installPath.indexOf(" ") > -1) {
            Logger.error(
                "[SF_Handler] - Install path must not contain spaces!"
            );
            throw new Error("Install Path Contains Spaces!");
        }

        try {
            Logger.info(
                `[SF_Handler] - Installing using ${Config.get(
                    "agent.sf.branch"
                )} branch`
            );
            const steamOutput = await SteamCMD.updateApp(
                1690800,
                installPath,
                Config.get("agent.sf.branch")
            );

            const installed = await this.isInstalled();
            if (installed) {
                Logger.info("[SF_Handler] - Installed SF Dedicated Server");
                await this.UpdateAgentInstalledState();
                Config.set(
                    "agent.sf.versions.installed",
                    Config.get("agent.sf.versions.available")
                );

                await Config.SendConfigToSSMCloud();
            }
        } catch (err) {
            Logger.error(
                "[SF_Handler] - Error Installing SF Dedicated Server!"
            );
            console.log(err);
        }
    };

    _RemoveSFServer = async () => {
        Logger.info("[SF_Handler] - Removing SF Dedicated Server");
        try {
            rimraf.sync(Config.get("agent.sfserver"));
            Logger.info("[SF_Handler] - Removed SF Dedicated Server");
        } catch (err) {
            Logger.error("[SF_Handler] - Remove SF Server Error");
            throw err;
        }
    };

    execSFSCmd = async (command) => {
        let ExeFileName = "FactoryServer.sh";

        if (process.platform == "win32") {
            ExeFileName = "FactoryServer.exe";
        }

        const SFSExe = path.join(Config.get("agent.sfserver"), ExeFileName);

        const fullCommand = '"' + SFSExe + '" ' + command;
        console.log(fullCommand);
        var sfprocess = childProcess.spawn(fullCommand, {
            shell: true,
            cwd: Config.get("agent.sfserver"),
            detached: true,
            stdio: "ignore",
        });

        // @todo: We get the close after this function has ended, I don't know how to capture return code properly here
        sfprocess.on("error", (err) => {
            Logger.debug(`[SF_Handler] - Child process with error ${err}`);
        });
        sfprocess.on("close", (code) => {
            Logger.debug(`[SF_Handler] - Child process on close ${code}`);
        });

        sfprocess.unref();
    };

    StartSFServer = async () => {
        Logger.info("[SF_Handler] - Starting SF Dedicated Server");
        const installed = await this.isInstalled();
        const running = await this.isServerRunning();

        if (!installed) {
            throw new Error("SF Server is not installed!");
        }

        if (running) {
            Logger.warn("[SF_Handler] - SF Server is already running");
            return;
        }
        const workerThreads = Config.get("agent.sf.worker_threads");
        const ServerQueryPort = 15777;
        const Port = 7777;
        const BeaconPort = 15000;

        try {
            await this.execSFSCmd(
                `?listen -Port=${Port} -ServerQueryPort=${ServerQueryPort} -BeaconPort=${BeaconPort} -unattended -MaxWorkerThreads=${workerThreads}`
            );
            await this.WaitTillServerStarted();
            Logger.info("[SF_Handler] - Server has started successfully");
        } catch (err) {
            Logger.error("[SF_Handler] - Server failed to start!");
            console.log(err);
            throw err;
        }
    };

    StopSFServer = async () => {
        Logger.info("[SF_Handler] - Stopping SF Dedicated Server");
        Cleanup.increaseCounter(1);

        const installed = await this.isInstalled();
        const running = await this.isServerRunning();

        if (!installed) {
            Cleanup.decreaseCounter(1);
            throw new Error("SF Server is not installed!");
        }

        if (!running) {
            Logger.warn("[SF_Handler] - SF Server is already stopped");
            Cleanup.decreaseCounter(1);
            return;
        }

        try {
            process.kill(this._processIds.pid2, "SIGTERM");
            process.kill(this._processIds.pid1, "SIGTERM");

            process.kill(this._processIds.pid2, "SIGINT");
            process.kill(this._processIds.pid1, "SIGINT");

            Logger.debug("Start waiting for server to stop");
            await this.WaitTillServerStopped();

            Logger.info("[SF_Handler] - Server has been stopped successfully");
            Cleanup.decreaseCounter(1);
        } catch (err) {
            Cleanup.decreaseCounter(1);
            Logger.error("[SF_Handler] - Server failed to stop!");
            console.log(err);
            throw err;
        }
    };

    KillSFServer = async () => {
        Logger.info("[SF_Handler] - Killing SF Dedicated Server");
        Cleanup.increaseCounter(1);

        const installed = await this.isInstalled();
        const running = await this.isServerRunning();

        if (!installed) {
            Cleanup.decreaseCounter(1);
            throw new Error("SF Server is not installed!");
        }

        if (!running) {
            Logger.warn("[SF_Handler] - SF Server is already stopped");
            Cleanup.decreaseCounter(1);
            return;
        }

        try {
            process.kill(this._processIds.pid1, "SIGKILL");
            process.kill(this._processIds.pid2, "SIGKILL");

            await this.WaitTillServerStopped();

            Logger.info("[SF_Handler] - Server has been killed successfully");
            Cleanup.decreaseCounter(1);
        } catch (err) {
            Cleanup.decreaseCounter(1);
            Logger.error("[SF_Handler] - Server failed to stop!");
            console.log(err);
            throw err;
        }
    };

    WaitTillServerStarted() {
        return new Promise((resolve, reject) => {
            let timeoutCounter = 0;
            let timeoutLimit = 30; // 30 Seconds

            const interval = setInterval(() => {
                this.isServerRunning().then((running) => {
                    if (timeoutCounter >= timeoutLimit) {
                        clearInterval(interval);
                        reject("Satisfactory server start timed out!");
                        return;
                    }

                    if (running) {
                        clearInterval(interval);
                        resolve();
                    } else {
                        timeoutCounter++;
                    }
                });
            }, 1000);
        });
    }

    WaitTillServerStopped() {
        return new Promise((resolve, reject) => {
            let timeoutCounter = 0;
            let timeoutLimit = 30; // 30 Seconds

            const interval = setInterval(() => {
                this.isServerRunning().then((running) => {
                    if (timeoutCounter >= timeoutLimit) {
                        clearInterval(interval);
                        reject("Satisfactory server start timed out!");
                        return;
                    }

                    Logger.debug("waiting for server to stop..");
                    console.log(this._processIds, timeoutCounter);

                    if (!running) {
                        clearInterval(interval);
                        resolve();
                    } else {
                        timeoutCounter++;
                    }
                });
            }, 1000);
        });
    }
}

const agentSFHandler = new AgentSFHandler();
module.exports = agentSFHandler;
