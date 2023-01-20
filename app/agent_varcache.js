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
    }

    setupLinuxVarCache() {
        super.get("homedir", path.join(osHomeDir, "SSMAgent"));
    }
}

const varCache = new AgentVarCache();
module.exports = varCache;
