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
        this._FicsitAPI = "https://api.ficsit.app";
    }

    init = async () => {
        this._ModsDir = path.join(
            Config.get("agent.sfserver"),
            "FactoryGame",
            "Mods"
        );

        this._TempModsDir = path.join(Config.get("agent.tempdir"), "mods");

        fs.ensureDirSync(this._ModsDir);
        fs.ensureDirSync(this._TempModsDir);

        await this.GetInstalledMods();
    };

    GetInstalledMods = async () => {
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

    HasInstalledVersion(modReference, version) {
        const installedmod = this._InstalledMods.find(
            (mod) =>
                mod._modReference == modReference && mod._version == version
        );

        return installedmod != null;
    }

    UpdateMod = async (modData) => {
        await this.InstallMod(modData, true);
    };

    InstallMod = async (modData, force = false) => {
        await this.GetInstalledMods();

        const SelectedVersionString = modData.modVersion;
        const ModInfo = modData.modInfo;
        const ModPlatform = VarCache.get("ModPlatform");

        Logger.info(
            `[ModManager] - Installing Mod (${ModInfo.modReference}) (${SelectedVersionString})`
        );

        if (
            this.HasInstalledVersion(
                ModInfo.modReference,
                SelectedVersionString
            ) &&
            !force
        ) {
            Logger.warn(
                `[ModManager] - Warning: Installing Mod (${ModInfo.modReference}) Skipped, Mod Already Installed!`
            );
            return;
        }

        if (ModInfo.versions == null || ModInfo.versions.length == 0) {
            Logger.error(
                `[ModManager] - Error: Installing Mod (${ModInfo.modReference}) No Versions Available!`
            );
            throw new Error(
                `Installing Mod (${ModInfo.modReference}) No Versions Available!`
            );
        }

        const SelectedVersion = ModInfo.versions.find(
            (v) => v.version == SelectedVersionString
        );

        if (SelectedVersion == null) {
            Logger.error(
                `[ModManager] - Error: Installing Mod (${ModInfo.modReference}) No Version Matching (${SelectedVersionString})`
            );
            throw new Error(
                `Installing Mod (${ModInfo.modReference}) No Version Matching (${SelectedVersionString})`
            );
        }

        await this.GetSMLVersionsFromAPI();

        if (SelectedVersion.arch == null) {
            Logger.error(
                `[ModManager] - Error: Installing Mod (${ModInfo.modReference}) No Server Version Matching (${SelectedVersionString})`
            );
            throw new Error(
                `Installing Mod (${ModInfo.modReference}) No Server Version Matching (${SelectedVersionString})`
            );
        }

        const SelectedArch = SelectedVersion.arch.find(
            (arch) => arch.platform == ModPlatform
        );

        if (SelectedArch == null) {
            Logger.error(
                `[ModManager] - Error: Installing Mod (${ModInfo.modReference}) No Server Version Matching (${SelectedVersionString}) (${ModPlatform})`
            );
            throw new Error(
                `Installing Mod (${ModInfo.modReference}) No Server Version Matching (${SelectedVersionString}) (${ModPlatform})`
            );
        }

        try {
            for (let i = 0; i < SelectedVersion.dependencies.length; i++) {
                const modDependency = SelectedVersion.dependencies[i];
                if (modDependency.mod_id == "SML") continue;

                await this.InstallModDependency(
                    modDependency.mod_id,
                    modDependency.condition
                );
            }
        } catch (err) {
            throw err;
        }

        try {
            await this.DownloadModVersion(
                ModInfo.modReference,
                SelectedVersion.version,
                SelectedArch.asset
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

    InstallModDependency = async (modReference, version) => {
        try {
            const modData = await this.GetModFromAPI(modReference);

            for (let i = 0; i < modData.versions.length; i++) {
                const modVersion = modData.versions[i];
                if (semver.satisfies(modVersion.version, version)) {
                    const data = {
                        modVersion: modVersion.version,
                        modInfo: modData,
                    };

                    await this.InstallMod(data);
                    return;
                }
            }
        } catch (err) {
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
        await this.GetInstalledMods();
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
}

const modManager = new AgentModManager();
module.exports = modManager;
