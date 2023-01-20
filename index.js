const Config = require("./app/agent_config");
const Logger = require("./app/agent_logger");
const AgentApp = require("./app/agent_app");

const VarCache = require("./app/agent_varcache");

Number.prototype.pad = function (width, z) {
    let n = this;
    z = z || "0";
    n = n + "";
    return n.length >= width ? n : new Array(width - n.length + 1).join(z) + n;
};

String.prototype.IsJsonString = () => {
    try {
        JSON.parse(this);
    } catch (e) {
        return false;
    }
    return true;
};

const Main = async () => {
    VarCache.init();
    Config.init();
    await Config.load();
    Logger.init();

    await AgentApp.init();
};

Main();
