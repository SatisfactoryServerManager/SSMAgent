const axios = require("axios");

const path = require("path");
const fs = require("fs-extra");
const FormData = require("form-data");

const backupFile = path.resolve(
    "D:\\Users\\DominicNew\\SSMAgent\\backup\\20221211_2150_Backup.zip"
);

const uploadBackup = async () => {
    const fileStream = fs.createReadStream(backupFile);

    const form = new FormData();
    // Pass file stream directly to form
    form.append("file", fileStream, "20221211_2150_Backup.zip");
    console.log(form.getHeaders());

    const url = "http://localhost:3000/api/agent/uploadbackup";

    const response = await axios.post(url, form, {
        headers: {
            "x-ssm-key": "ABC123",
            ...form.getHeaders(),
            Authorization: "Bearer ...", // optional
        },
    });
};

uploadBackup();
