const iConfig = require("mrhid6utils").Config;

const platform = process.platform;
const fs = require("fs-extra");
const path = require("path");

const Config = require("./agent_config");

class EngineConfig extends iConfig {
    constructor(configDir) {
        super({
            useExactPath: true,
            configBaseDirectory: configDir,
            configName: "Engine",
            configType: "ini",
            createConfig: true,
        });
    }

    setDefaultValues = async () => {
        super.set("/Script/Engine.Player.ConfiguredInternetSpeed", 104857600);
        super.set("/Script/Engine.Player.ConfiguredLanSpeed", 104857600);

        super.set(
            "/Script/OnlineSubsystemUtils.IpNetDriver.NetServerMaxTickRate",
            120
        );
        super.set(
            "/Script/OnlineSubsystemUtils.IpNetDriver.MaxNetTickRate",
            400
        );
        super.set(
            "/Script/OnlineSubsystemUtils.IpNetDriver.MaxInternetClientRate",
            104857600
        );
        super.set(
            "/Script/OnlineSubsystemUtils.IpNetDriver.MaxClientRate",
            104857600
        );
        super.set(
            "/Script/OnlineSubsystemUtils.IpNetDriver.LanServerMaxTickRate",
            120
        );
        super.set(
            "/Script/OnlineSubsystemUtils.IpNetDriver.InitialConnectTimeout",
            300
        );
        super.set(
            "/Script/OnlineSubsystemUtils.IpNetDriver.ConnectionTimeout",
            300
        );

        super.set(
            "/Script/SocketSubsystemEpic.EpicNetDriver.MaxClientRate",
            104857600
        );
        super.set(
            "/Script/SocketSubsystemEpic.EpicNetDriver.MaxInternetClientRate",
            104857600
        );
    };
}

class GameConfig extends iConfig {
    constructor(configDir) {
        super({
            useExactPath: true,
            configBaseDirectory: configDir,
            configName: "Game",
            configType: "ini",
            createConfig: true,
        });
    }

    setDefaultValues = async () => {
        super.set(
            "/Script/Engine.GameNetworkManager.TotalNetBandwidth",
            104857600
        );
        super.set(
            "/Script/Engine.GameNetworkManager.MaxDynamicBandwidth",
            104857600
        );
        super.set(
            "/Script/Engine.GameNetworkManager.MinDynamicBandwidth",
            104857600
        );

        super.get("/Script/Engine.GameSession.MaxPlayers", 20);
    };
}

class AgentServerConfigManager {
    constructor() {}

    init = async () => {
        let PlatformFolder = "LinuxServer";

        if (platform == "win32") {
            PlatformFolder = "WindowsServer";
        }

        const configDir = path.join(
            Config.get("agent.sfserver"),
            "FactoryGame",
            "Saved",
            "Config",
            PlatformFolder
        );
        fs.ensureDirSync(configDir);

        this._GameConfig = new GameConfig(configDir);
        this._EngineConfig = new EngineConfig(configDir);

        await this._GameConfig.load();
        await this._EngineConfig.load();
    };

    getGameConfig() {
        return this._GameConfig;
    }

    getEngineConfig() {
        return this._EngineConfig;
    }
}

const agentServerConfigManager = new AgentServerConfigManager();
module.exports = agentServerConfigManager;
