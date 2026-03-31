#!/usr/bin/env node
"use strict";

if (process.env.REPO_FALCON_SKIP_INSTALL) {
  console.log("repo-falcon: skipping binary download (REPO_FALCON_SKIP_INSTALL is set)");
  process.exit(0);
}

require("./install").ensureBinary().catch((err) => {
  console.error(`repo-falcon: ${err.message}`);
  process.exit(1);
});
