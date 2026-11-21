import assert from "node:assert/strict";
import test from "node:test";
import { bytes } from "../src/fileSize.ts";

test("file sizes use byte-appropriate units without rounding small scripts to zero MiB", () => {
 assert.equal(bytes(0), "0 B");
 assert.equal(bytes(1), "1 B");
 assert.equal(bytes(1023), "1023 B");
 assert.equal(bytes(1024), "1.0 KiB");
 assert.equal(bytes(5120), "5.0 KiB");
 assert.equal(bytes(1048576), "1.0 MiB");
 assert.equal(bytes(1073741824), "1.0 GiB");
 assert.equal(bytes(1099511627776), "1.0 TiB");
});
