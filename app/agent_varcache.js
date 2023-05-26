const Mrhid6Utils = require("mrhid6utils");
const iVarCache = Mrhid6Utils.VarCache;

const path = require("path");
const fs = require("fs-extra");

const osHomeDir = require("os").homedir();
const { getDataHome, getHomeFolder } = require("platform-folders");

class AgentVarCache extends iVarCache {
    init() {
        super.init();

        fs.ensureDirSync(super.get("homedir"));
    }

    setupWindowsVarCache() {
        super.get("homedir", path.join(osHomeDir, "SSMAgent"));
        super.set("ModPlatform", "WindowsServer");
        super.set("PlatformFolder", "WindowsServer");

        let localAppdata = path.resolve(
            getDataHome() + "/../local/FactoryGame"
        );

        const SaveFolder = path.join(
            localAppdata,
            "Saved",
            "SaveGames",
            "server"
        );

        super.get("savedir", SaveFolder);
    }

    setupLinuxVarCache() {
        super.get("homedir", path.join(osHomeDir, "SSMAgent"));
        super.set("ModPlatform", "LinuxServer");
        super.set("PlatformFolder", "LinuxServer");

        let localAppdata = path.resolve(
            getHomeFolder() + "/.config/Epic/FactoryGame"
        );
        const SaveFolder = path.join(
            localAppdata,
            "Saved",
            "SaveGames",
            "server"
        );

        super.get("savedir", SaveFolder);
    }
}

const varCache = new AgentVarCache();
module.exports = varCache;
