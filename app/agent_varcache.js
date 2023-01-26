const Mrhid6Utils = require("mrhid6utils");
const iVarCache = Mrhid6Utils.VarCache;

const path = require("path");
const fs = require("fs-extra");

const osHomeDir = require("os").homedir();

class AgentVarCache extends iVarCache {
    init() {
        super.init();

        fs.ensureDirSync(super.get("homedir"));
    }

    setupWindowsVarCache() {
        super.get("homedir", path.join(osHomeDir, "SSMAgent"));
        super.set("ModPlatform", "WindowsServer");
        super.set("PlatformFolder", "WindowsServer");
    }

    setupLinuxVarCache() {
        super.get("homedir", path.join(osHomeDir, "SSMAgent"));
        super.set("ModPlatform", "LinuxServer");
        super.set("PlatformFolder", "LinuxServer");
    }
}

const varCache = new AgentVarCache();
module.exports = varCache;
