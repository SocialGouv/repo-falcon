#!/usr/bin/env node
"use strict";

const { execFileSync } = require("child_process");
const fs = require("fs");
const path = require("path");

const ext = process.platform === "win32" ? ".exe" : "";
const bin = path.join(__dirname, "bin", "falcon" + ext);

async function main() {
  // If binary is missing (postinstall failed or --ignore-scripts), download it now
  if (!fs.existsSync(bin)) {
    await require("./install").ensureBinary();
  }
  try {
    execFileSync(bin, process.argv.slice(2), { stdio: "inherit" });
  } catch (e) {
    process.exit(e.status ?? 1);
  }
}

main().catch((e) => {
  console.error(`repo-falcon: ${e.message}`);
  process.exit(1);
});
