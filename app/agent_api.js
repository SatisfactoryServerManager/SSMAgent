const axios = require("axios");
const Config = require("./agent_config");
const fs = require("fs-extra");
const path = require("path");
const { getDataHome, getHomeFolder } = require("platform-folders");
const platform = process.platform;

class AgentAPI {
    constructor() {}

    remoteRequestGET = async (endpoint) => {
        const reqconfig = {
            headers: {
                "x-ssm-key": Config.get("agent.ssmcloud.apikey"),
            },
        };
        const url = `${Config.get("agent.ssmcloud.url")}/${endpoint}`;

        try {
            const res = await axios.get(url, reqconfig);

            const data = res.data;

            if (!data.success) {
                throw new Error("Request returned an error: " + data.error);
            } else {
                return data;
            }
        } catch (err) {
            const data = err.response.data;

            if (!data.success) {
                throw new Error("Request returned an error: " + data.error);
            }
        }
    };
    remoteRequestPOST = async (endpoint, requestdata) => {
        const reqconfig = {
            headers: {
                "x-ssm-key": Config.get("agent.ssmcloud.apikey"),
            },
        };

        const url = `${Config.get("agent.ssmcloud.url")}/${endpoint}`;

        //console.log(url, requestdata);

        try {
            const res = await axios.post(url, requestdata, reqconfig);
            const data = res.data;

            if (!data.success) {
                throw new Error("Request returned an error: " + data.error);
            } else {
                return data;
            }
        } catch (err) {
            throw err;
        }
    };

    DownloadAgentSaveFile(fileName) {
        return new Promise((resolve, reject) => {
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

            const outputFile = path.join(SaveFolder, fileName);
            const writer = fs.createWriteStream(outputFile);

            const reqconfig = {
                headers: {
                    "x-ssm-key": Config.get("agent.ssmcloud.apikey"),
                },
                responseType: "stream",
            };

            const url = `${Config.get(
                "agent.ssmcloud.url"
            )}/api/agent/saves/download/${fileName}`;

            axios.get(url, reqconfig).then((res) => {
                res.data.pipe(writer);
                let error = null;

                writer.on("error", (err) => {
                    error = err;
                    writer.close();
                    reject(err);
                });

                writer.on("close", (err) => {
                    if (!error) {
                        resolve(outputFile);
                    }
                });
            });
        });
    }
}
const agentApi = new AgentAPI();
module.exports = agentApi;
