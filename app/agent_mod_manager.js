const Logger = require("./agent_logger");
const Config = require("./agent_config");
const VarCache = require("./agent_varcache");
const AgentAPI = require("./agent_api");

const rra = require("recursive-readdir-async");
const rimraf = require("rimraf");

const fs = require("fs-extra");
const path = require("path");
const semver = require("semver");
const axios = require("axios");
const extractZip = require("extract-zip");

class InstalledMod {
    constructor(modReference, version, modPath) {
        this._modReference = modReference;
        this._version = version;
        this._modPath = modPath;
        this._uPluginPath = path.join(
            this._modPath,
            `${this._modReference}.uplugin`
        );
    }

    GetModReference() {
        return this._modReference;
    }

    GetVersion() {
        return this._version;
    }
}

class AgentModManager {
    constructor() {
        this._InstalledMods = [];
    }

    init = async () => {
        this._FicsitAPI = Config.get("agent.mods.api");

        Logger.info("[ModManager] - Initialising Mod Manager ...");
        this._ModsDir = path.join(
            Config.get("agent.sfserver"),
            "FactoryGame",
            "Mods"
        );

        this._TempModsDir = path.join(Config.get("agent.tempdir"), "mods");

        fs.ensureDirSync(this._ModsDir);
        fs.ensureDirSync(this._TempModsDir);

        await this.Task_CompareModState();

        setInterval(async () => {
            try {
                await this.Task_CompareModState();
            } catch (err) {
                console.log(err);
            }
        }, 30000);

        Logger.info("[ModManager] - Initialised Mod Manager");
    };

    GetAgentModState = async () => {
        try {
            const res = await AgentAPI.remoteRequestGET("api/agent/modState");
            this._ModState = res.modState;
        } catch (err) {
            Logger.error("Failed to get Agent Mod State!");
        }
    };

    GetInstalledMods = async () => {
        fs.ensureDirSync(this._ModsDir);
        fs.ensureDirSync(this._TempModsDir);

        this._InstalledMods = [];

        const fileList = fs
            .readdirSync(this._ModsDir, { withFileTypes: true })
            .filter((dirent) => dirent.isDirectory())
            .map((dirent) => dirent.name);

        for (let i = 0; i < fileList.length; i++) {
            const modFolder = fileList[i];

            const modFolderPath = path.join(this._ModsDir, modFolder);
            const modUPlugin = path.join(modFolderPath, `${modFolder}.uplugin`);
            if (!fs.existsSync(modUPlugin)) {
                Logger.warn(
                    `[ModManager] - Skipping mod ${modFolder} - no UPlugin`
                );
                continue;
            }

            let uPluginJson;
            try {
                uPluginJson = JSON.parse(fs.readFileSync(modUPlugin));
            } catch (err) {}

            if (uPluginJson.SemVersion != null) {
                Logger.debug(
                    `[ModManager] - Found Mod (${uPluginJson.FriendlyName}) with version (${uPluginJson.SemVersion})`
                );

                const instalModObj = new InstalledMod(
                    modFolder,
                    uPluginJson.SemVersion,
                    modFolderPath
                );
                this._InstalledMods.push(instalModObj);
            }
        }
    };

    CompareModState = async () => {
        Logger.info("[ModManager] - Comparing Mod State..");

        for (let i = 0; i < this._ModState.selectedMods.length; i++) {
            const selectedMod = this._ModState.selectedMods[i];
            const isInstalled = this.HasInstalledVersion(
                selectedMod.mod.modReference,
                selectedMod.desiredVersion
            );

            if (isInstalled) {
                if (!selectedMod.installed) {
                    Logger.debug(
                        `[ModManager] - ModState - ${selectedMod.mod.modReference} installed!`
                    );

                    const theInstalledMod = this.GetInstalledVersion(
                        selectedMod.mod.modReference
                    );

                    selectedMod.installed = true;
                    selectedMod.installedVersion = theInstalledMod.GetVersion();
                }
            } else {
                selectedMod.installed = false;
            }
        }
    };

    TryInstallModsFromState = async () => {
        for (let i = 0; i < this._ModState.selectedMods.length; i++) {
            const selectedMod = this._ModState.selectedMods[i];

            if (selectedMod.installed == true) continue;

            await this._InstallMod(selectedMod.mod, selectedMod.desiredVersion);
        }

        for (let i = 0; i < this._InstalledMods.length; i++) {
            const installedMod = this._InstalledMods[i];

            const selectedMod = this._ModState.selectedMods.find(
                (sm) => sm.mod.modReference == installedMod.GetModReference()
            );

            if (selectedMod == null) {
                if (installedMod.GetModReference() != "SML") {
                    await this.UninstallMod(installedMod.GetModReference());
                }
            }
        }
    };

    Task_CompareModState = async () => {
        const oldModState = this._ModState;
        await this.GetAgentModState();

        if (JSON.stringify(oldModState) === JSON.stringify(this._ModState)) {
            return;
        }

        await this.GetInstalledMods();
        await this.CompareModState();
        await this.TryInstallModsFromState();
        await this.GetInstalledMods();
        await this.CompareModState();
        await this.SendModStateToAPI();
    };

    SendModStateToAPI = async () => {
        try {
            const res = await AgentAPI.remoteRequestPOST(
                "api/agent/modstate",
                this._ModState
            );
        } catch (err) {
            throw err;
        }
    };

    HasInstalledVersion(modReference, version) {
        const installedmod = this._InstalledMods.find(
            (mod) =>
                mod._modReference == modReference && mod._version == version
        );

        return installedmod != null;
    }

    GetInstalledVersion(modReference) {
        return this._InstalledMods.find(
            (mod) => mod._modReference == modReference
        );
    }

    UpdateMod = async (modData) => {
        await this.InstallMod(modData, true);
    };

    InstallMod = async (modData, force = false) => {
        fs.ensureDirSync(this._ModsDir);
        fs.ensureDirSync(this._TempModsDir);

        await this.GetInstalledMods();
        await this._InstallMod(modData, force);
        await this.SendInstalledModList();
    };

    _InstallMod = async (Mod, DesiredVersion, force = false) => {
        const ModPlatform = VarCache.get("ModPlatform");

        Logger.info(
            `[ModManager] - Installing Mod (${Mod.modReference}) (${DesiredVersion})`
        );

        if (
            this.HasInstalledVersion(Mod.modReference, DesiredVersion) &&
            !force
        ) {
            Logger.warn(
                `[ModManager] - Warning: Installing Mod (${Mod.modReference}) Skipped, Mod Already Installed!`
            );
            return;
        }

        if (Mod.versions == null || Mod.versions.length == 0) {
            Logger.error(
                `[ModManager] - Error: Installing Mod (${Mod.modReference}) No Versions Available!`
            );
            throw new Error(
                `Installing Mod (${Mod.modReference}) No Versions Available!`
            );
        }

        const SelectedVersion = Mod.versions.find(
            (v) => v.version == DesiredVersion
        );

        if (SelectedVersion == null) {
            Logger.error(
                `[ModManager] - Error: Installing Mod (${Mod.modReference}) No Version Matching (${DesiredVersion})`
            );
            throw new Error(
                `Installing Mod (${Mod.modReference}) No Version Matching (${DesiredVersion})`
            );
        }

        await this.GetSMLVersionsFromAPI();

        let usingEXP = false;

        if (SelectedVersion.arch == null && SelectedVersion.targets == null) {
            Logger.error(
                `[ModManager] - Error: Installing Mod (${Mod.modReference}) No Server Version Matching (${DesiredVersion})`
            );
            throw new Error(
                `Installing Mod (${Mod.modReference}) No Server Version Matching (${DesiredVersion})`
            );
        }

        let SelectedArch = null;
        if (SelectedVersion.arch) {
            SelectedArch = SelectedVersion.arch.find(
                (arch) => arch.platform == ModPlatform
            );
        }

        if (SelectedVersion.targets) {
            usingEXP = true;
            SelectedArch = SelectedVersion.targets.find(
                (target) => target.targetName == ModPlatform
            );
        }

        if (SelectedArch == null) {
            Logger.error(
                `[ModManager] - Error: Installing Mod (${Mod.modReference}) No Server Version Matching (${DesiredVersion}) (${ModPlatform})`
            );
            throw new Error(
                `Installing Mod (${Mod.modReference}) No Server Version Matching (${DesiredVersion}) (${ModPlatform})`
            );
        }

        try {
            await this.DownloadModVersion(
                Mod.modReference,
                SelectedVersion.version,
                usingEXP ? SelectedArch.link : SelectedArch.asset
            );
        } catch (err) {
            console.log(err);
            throw err;
        }

        try {
            this.InstallSML(SelectedVersion.sml_version, force);
        } catch (err) {
            console.log(err);
            throw err;
        }
    };

    DownloadModVersion = async (modReference, version, versionAsset) => {
        const ModPath = path.join(this._ModsDir, modReference);
        const ZipFile = `${modReference}-${version}.zip`;
        const ZipFilePath = path.join(this._TempModsDir, ZipFile);

        let skipDownload = false;
        if (fs.existsSync(ZipFilePath)) {
            skipDownload = true;
        }

        if (fs.existsSync(ModPath)) {
            rimraf.sync(ModPath);
        }

        const downloadUrl = `${this._FicsitAPI}${versionAsset}`;

        try {
            if (!skipDownload) {
                Logger.info(
                    `[ModManager] - Downloading Mod (${modReference}) (${versionAsset})`
                );
                const responseStream = await axios.get(downloadUrl, {
                    responseType: "stream",
                });

                const tempFileWriteStream = fs.createWriteStream(ZipFilePath);

                responseStream.data.pipe(tempFileWriteStream);
                await new Promise((resolve) => {
                    tempFileWriteStream.on("finish", resolve);
                });

                Logger.info(`[ModManager] - Downloaded Mod (${modReference})`);
            }

            Logger.info(`[ModManager] - Extracting Mod (${modReference})`);
            await this.ExtractModArchive(ZipFilePath, ModPath);
            Logger.info(`[ModManager] - Extracted Mod (${modReference})`);
        } catch (err) {
            console.log(err);
        }
    };

    ExtractModArchive = async (pathToArchive, targetDirectory) => {
        await extractZip(pathToArchive, {
            dir: targetDirectory,
        });
    };

    InstallSML = async (RequestedSMLVersion, force = false) => {
        const SMLInstalledMod = this._InstalledMods.find(
            (mod) => mod._modReference == "SML"
        );

        if (SMLInstalledMod) {
            if (
                semver.satisfies(
                    SMLInstalledMod._version,
                    RequestedSMLVersion
                ) &&
                !force
            ) {
                Logger.warn(
                    `[ModManager] - Installing SML Skipped - Installed SML Satisfies Requested (${RequestedSMLVersion})`
                );
                return;
            }
        }

        const Version = semver.coerce(RequestedSMLVersion);
        await this.DownloadSML(`v${Version}`);

        await this.GetInstalledMods();
    };

    DownloadSML = async (Version) => {
        const SMLModPath = path.join(this._ModsDir, "SML");

        const ZipFile = `SML-${Version}.zip`;
        const ZipFilePath = path.join(this._TempModsDir, ZipFile);

        let skipDownload = false;
        if (fs.existsSync(ZipFilePath)) {
            skipDownload = true;
        }

        if (fs.existsSync(SMLModPath)) {
            rimraf.sync(SMLModPath);
        }

        const DownloadURL = `https://github.com/satisfactorymodding/SatisfactoryModLoader/releases/download/${Version}/SML.zip`;

        try {
            if (!skipDownload) {
                Logger.info(`[ModManager] - Downloading SML (${Version})`);
                const responseStream = await axios.get(DownloadURL, {
                    responseType: "stream",
                });

                const tempFileWriteStream = fs.createWriteStream(ZipFilePath);

                responseStream.data.pipe(tempFileWriteStream);
                await new Promise((resolve) => {
                    tempFileWriteStream.on("finish", resolve);
                });

                Logger.info(`[ModManager] - Downloaded SML (${Version})`);
            }

            Logger.info(`[ModManager] - Extracting SML (${Version})`);
            await this.ExtractModArchive(ZipFilePath, SMLModPath);
            Logger.info(`[ModManager] - Extracted SML (${Version})`);
        } catch (err) {
            console.log(err);
        }
    };

    GetSMLVersionsFromAPI = async () => {
        try {
            const res = await AgentAPI.remoteRequestGET(
                "api/agent/getsmlversions"
            );

            this._smlVersions = res.data;
        } catch (err) {
            console.log(err);
        }
    };

    GetModFromAPI = async (modReference) => {
        try {
            const res = await AgentAPI.remoteRequestGET(
                `api/agent/mod/${modReference}`
            );

            return res.data;
        } catch (err) {
            throw err;
        }
    };

    SendInstalledModList = async () => {
        try {
            await AgentAPI.remoteRequestPOST("api/agent/mods", {
                mods: this._InstalledMods,
            });
        } catch (err) {
            console.log(err);
        }
    };

    UninstallMod = async (modReference) => {
        Logger.info(`[ModManager] - Uninstalling mod ${modReference}`);
        const installedMod = this.GetInstalledVersion(modReference);

        if (fs.existsSync(installedMod._modPath)) {
            rimraf.sync(installedMod._modPath);
            Logger.info(
                `[ModManager] - Successfully Uninstalled mod ${modReference}`
            );
        }
    };
}

const modManager = new AgentModManager();
module.exports = modManager;
