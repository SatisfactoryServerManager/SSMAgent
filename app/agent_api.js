const axios = require("axios");
const Config = require("./agent_config");
const fs = require("fs-extra");
const path = require("path");

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
            throw err;
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
}
const agentApi = new AgentAPI();
module.exports = agentApi;
