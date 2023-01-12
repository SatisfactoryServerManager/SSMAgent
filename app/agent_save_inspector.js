const SaveManager = require("./agent_save_manager");
const SFHandler = require("./agent_sf_handler");

const Logger = require("./agent_logger");

const AgentAPI = require("./agent_api");

var fs = require("fs-extra");
const path = require("path");
const sfSavToJson = require("satisfactory-json").sav2json;

class SaveInspector {
    constructor() {
        this._ParsedSaveData = {};
    }
    init = async () => {
        await this.StartPolling();
    };

    StartPolling = async () => {
        await this.inspectSaveFile();

        setInterval(async () => {
            await this.inspectSaveFile();
        }, 1 * 60 * 1000);
    };

    inspectSaveFile = async () => {
        const running = await SFHandler.isServerRunning();
        if (running) return;

        const AllSaveFiles = await SaveManager.GetAllSaveFiles();
        if (AllSaveFiles.length == 0) return;

        const SaveFile = AllSaveFiles[0];

        if (!fs.existsSync(SaveFile.path)) return;

        console.log(SaveFile);

        this._ParsedSaveData.sessionName = SaveFile.sessionName;

        let SavefileData = {};

        try {
            Logger.debug("[Stats_Manager] - Loading Save Data...");
            var data = fs.readFileSync(SaveFile.path);
            SavefileData = await sfSavToJson(data);
            Logger.debug("[Stats_Manager] - Save Data Loaded");
        } catch (err) {
            throw new Error(err.message);
        }

        try {
            Logger.debug("[Stats_Manager] - Parsing Save Data...");
            await this.GetFoundationsCountFromSaveData(SavefileData);
            await this.GetConveyorCountFromSaveData(SavefileData);
            await this.GetFactoriesCountFromSaveData(SavefileData);
            await this.GetPipelineCountFromSaveData(SavefileData);
            await this.GetPlaytimeFromSaveData(SavefileData);
            await this.GetGamePhaseFromSaveData(SavefileData);
            Logger.debug("[Stats_Manager] - Save Data Parsed!");
        } catch (err) {
            throw err;
        }

        try {
            this.UploadParsedSaveDataToAPI();
        } catch (err) {
            throw err;
        }
    };

    GetFoundationsCountFromSaveData = async (SaveData) => {
        const actors = SaveData.actors;
        const filteredActors = actors.filter((a) =>
            a.pathName.includes("Foundation")
        );

        this._ParsedSaveData.Foundations = filteredActors.length;
    };
    GetConveyorCountFromSaveData = async (SaveData) => {
        const actors = SaveData.actors;
        const filteredActors = actors.filter((a) =>
            a.pathName.includes("ConveyorBelt")
        );

        this._ParsedSaveData.Conveyors = filteredActors.length;
    };

    GetFactoriesCountFromSaveData = async (SaveData) => {
        const actors = SaveData.actors;
        const filteredActors = actors.filter(
            (a) =>
                a.pathName.includes("Constructor") ||
                a.pathName.includes("Assembler") ||
                a.pathName.includes("Manufacturer") ||
                a.pathName.includes("Blender") ||
                a.pathName.includes("Foundry") ||
                a.pathName.includes("Smelter") ||
                a.pathName.includes("HadronCollider") ||
                a.pathName.includes("Packager") ||
                a.pathName.includes("Oil_Refinery")
        );

        this._ParsedSaveData.Factories = filteredActors.length;
    };

    GetPipelineCountFromSaveData = async (SaveData) => {
        const actors = SaveData.actors;
        const filteredActors = actors.filter(
            (a) =>
                a.pathName.includes("Pipeline_C") ||
                a.pathName.includes("PipelineMK2_C")
        );

        this._ParsedSaveData.Pipes = filteredActors.length;
    };

    GetPlaytimeFromSaveData = async (SaveData) => {
        this._ParsedSaveData.PlayTime = SaveData.playDurationSeconds;
    };

    GetGamePhaseFromSaveData = async (SaveData) => {
        const actors = SaveData.actors;
        const GamePhaseManager = actors.find((a) =>
            a.pathName.includes("GamePhaseManager")
        );

        const Phases = {
            EGP_EarlyGame: 0,
            EGP_MidGame: 1,
            EGP_LateGame: 2,
            EGP_EndGame: 3,
            EGP_FoodCourt: 4,
            EGP_Victory: 5,
        };

        const PhasesNames = [
            "Establishing Phase",
            "Development Phase",
            "Expansion Phase",
            "Retention Phase",
            "Food Court",
            "Victory!",
        ];

        let Phase = Phases.EGP_EarlyGame;
        let PhaseObj = {
            enum: Phase,
            name: PhasesNames[Phase],
        };
        if (GamePhaseManager != null) {
            Phase =
                Phases[
                    `${GamePhaseManager.entity.properties[0].value.valueName}`
                ];

            PhaseObj = {
                enum: Phase,
                name: PhasesNames[Phase],
            };
        }

        this._ParsedSaveData.GamePhase = PhaseObj;
    };

    UploadParsedSaveDataToAPI = async () => {
        try {
            await AgentAPI.remoteRequestPOST("api/agent/savestats", {
                data: this._ParsedSaveData,
            });
        } catch (err) {
            console.log(err);
        }
    };
}

const saveInspector = new SaveInspector();
module.exports = saveInspector;
