const Config = require("./app/agent_config");
const Logger = require("./app/agent_logger");
const AgentApp = require("./app/agent_app");

const Main = async () => {
    await Config.load();
    Logger.init();

    await AgentApp.init();
};

Main();
