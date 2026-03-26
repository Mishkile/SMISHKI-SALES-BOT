import fs from "fs";
import path from "path";

// Paths relative to src/tests/
const LOCALS_PATH = path.join(__dirname, "../../locals.json");
const SRC_PATH = path.join(__dirname, "../");

interface Locals {
    [lang: string]: { [key: string]: string };
}

function getFiles(dir: string, ext: string): string[] {
    let files: string[] = [];
    const items = fs.readdirSync(dir, { withFileTypes: true });
    for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory()) {
            files = files.concat(getFiles(fullPath, ext));
        } else if (item.name.endsWith(ext)) {
            files.push(fullPath);
        }
    }
    return files;
}

function runCheck() {
    console.log("[INFO] Checking locals.json...");

    if (!fs.existsSync(LOCALS_PATH)) {
        console.error("[ERROR] locals.json not found!");
        process.exit(1);
    }

    const locals: Locals = JSON.parse(fs.readFileSync(LOCALS_PATH, "utf-8"));
    const languages = Object.keys(locals);

    // 1. Collect all unique keys from ALL languages
    const allKeys = new Set<string>();
    languages.forEach(lang => {
        Object.keys(locals[lang]).forEach(key => allKeys.add(key));
    });
    const sortedKeys = Array.from(allKeys).sort();

    let hasError = false;

    // 2. Bidirectional Check: Ensure every language has every key
    console.log(`\n[INFO] verifying consistency across ${languages.length} languages: ${languages.join(", ")}`);

    for (const lang of languages) {
        const currentKeys = Object.keys(locals[lang]);
        const missing = sortedKeys.filter(k => !currentKeys.includes(k));

        if (missing.length > 0) {
            console.error(`  [ERROR] Language '${lang}' is missing keys: ${missing.join(", ")}`);
            hasError = true;
        }
    }
    if (!hasError) console.log("  [SUCCESS] All languages have matching keys.");

    // 3. Check for unused keys
    console.log("\n[INFO] Scanning source code for usage...");
    const tsFiles = getFiles(SRC_PATH, ".ts");
    const fileContents = tsFiles.map(f => fs.readFileSync(f, "utf-8")).join("\n");
    const unusedKeys: string[] = [];

    for (const key of sortedKeys) {
        // Matches .keyName or "keyName" or 'keyName'
        const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
        const regex = new RegExp(`\\.${escapedKey}\\b|\\[["']${escapedKey}["']\\]`);
        if (!regex.test(fileContents)) {
            unusedKeys.push(key);
        }
    }

    if (unusedKeys.length > 0) {
        console.warn(`  [WARN] Potentially unused keys (${unusedKeys.length}):\n     - ${unusedKeys.join("\n     - ")}`);
    } else {
        console.log("  [SUCCESS] All keys appear to be used.");
    }

    if (hasError) process.exit(1);
}

runCheck();