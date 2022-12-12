const Config = require("./app/agent_config");
const Logger = require("./app/agent_logger");
const AgentApp = require("./app/agent_app");

Number.prototype.pad = function (width, z) {
    let n = this;
    z = z || "0";
    n = n + "";
    return n.length >= width ? n : new Array(width - n.length + 1).join(z) + n;
};

const Main = async () => {
    await Config.load();
    Logger.init();

    await AgentApp.init();
};

Main();
