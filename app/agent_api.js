const Config = require("./agent_config");
const fs = require("fs-extra");
const path = require("path");
const { getDataHome, getHomeFolder } = require("platform-folders");
const platform = process.platform;

const fetch = require("isomorphic-fetch");

const { Readable } = require("stream");
const { finished } = require("stream/promises");

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
            const res = await fetch(url, { headers: reqconfig.headers });

            const data = await res.json();

            if (!data.success) {
                throw new Error("Request returned an error: " + data.error);
            } else {
                return data;
            }
        } catch (err) {
            throw err;
        }
    };
    remoteRequestPOST = async (endpoint, requestdata) => {
        const reqconfig = {
            headers: {
                "x-ssm-key": Config.get("agent.ssmcloud.apikey"),
                "Content-Type": "application/json",
            },
        };

        const url = `${Config.get("agent.ssmcloud.url")}/${endpoint}`;

        console.log(url, requestdata);

        try {
            const res = await fetch(url, {
                method: "POST",
                body: JSON.stringify(requestdata),
                headers: reqconfig.headers,
            });
            const data = await res.json();

            if (!data.success) {
                throw new Error("Request returned an error: " + data.error);
            } else {
                return data;
            }
        } catch (err) {
            throw err;
        }
    };

    DownloadAgentSaveFile = async (fileName) => {
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

        try {
            const req = request({ url, headers: reqconfig.headers });
            await new Promise((resolve, reject) => {
                req.pipe(tempFileWriteStream);
                req.on("error", (err) => {
                    reject(err);
                });
                tempFileWriteStream.on("finish", function () {
                    resolve();
                });
            });
        } catch (err) {
            throw err;
        }
    };
}
const agentApi = new AgentAPI();
module.exports = agentApi;
