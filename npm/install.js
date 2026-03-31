#!/usr/bin/env node
"use strict";

const fs = require("fs");
const path = require("path");
const crypto = require("crypto");

const PLATFORM_MAP = { darwin: "darwin", linux: "linux", win32: "windows" };
const ARCH_MAP = { x64: "amd64", arm64: "arm64" };
const REPO = "SocialGouv/repo-falcon";

/**
 * Ensures the falcon binary is downloaded and available.
 * Returns the absolute path to the binary.
 */
async function ensureBinary() {
  const goos = PLATFORM_MAP[process.platform];
  const goarch = ARCH_MAP[process.arch];
  if (!goos || !goarch) {
    throw new Error(`unsupported platform ${process.platform}/${process.arch}`);
  }

  const { version } = require("./package.json");
  const ext = process.platform === "win32" ? ".exe" : "";
  const binDir = path.join(__dirname, "bin");
  const binPath = path.join(binDir, `falcon${ext}`);
  const versionFile = path.join(binDir, ".version");

  // Skip if already installed at this version
  if (fs.existsSync(binPath) && fs.existsSync(versionFile)) {
    const installed = fs.readFileSync(versionFile, "utf8").trim();
    if (installed === version) {
      return binPath;
    }
  }

  const filename = `falcon-${goos}-${goarch}${ext}`;
  const baseUrl = `https://github.com/${REPO}/releases/download/v${version}`;
  const binaryUrl = `${baseUrl}/${filename}`;
  const checksumUrl = `${binaryUrl}.sha256`;

  console.error(`repo-falcon: downloading v${version} (${goos}/${goarch})...`);

  const binaryBuf = await download(binaryUrl);

  // Verify checksum
  if (process.env.REPO_FALCON_SKIP_CHECKSUM) {
    console.error("repo-falcon: skipping checksum verification (REPO_FALCON_SKIP_CHECKSUM is set)");
  } else {
    const checksumBuf = await download(checksumUrl);
    const expectedHash = checksumBuf.toString("utf8").split(/\s+/)[0];
    const actualHash = crypto.createHash("sha256").update(binaryBuf).digest("hex");
    if (actualHash !== expectedHash) {
      throw new Error(`checksum mismatch (expected ${expectedHash}, got ${actualHash})`);
    }
    console.error("repo-falcon: checksum verified");
  }

  // Write binary
  fs.mkdirSync(binDir, { recursive: true });
  fs.writeFileSync(binPath, binaryBuf);
  fs.chmodSync(binPath, 0o755);
  fs.writeFileSync(versionFile, version);

  console.error(`repo-falcon: v${version} installed`);
  return binPath;
}

async function download(url) {
  const res = await fetch(url, { redirect: "follow" });
  if (!res.ok) {
    throw new Error(`Failed to download ${url}: ${res.status} ${res.statusText}`);
  }
  return Buffer.from(await res.arrayBuffer());
}

module.exports = { ensureBinary };
