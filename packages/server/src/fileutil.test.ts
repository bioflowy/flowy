import { describe, it, expect } from 'vitest';
import { get_js_engine } from './sandbox';
import * as fs from 'node:fs';
import { removeIgnorePermissionError } from './fileutils';

describe('rmove directory recursively',() => {
  it('rmove directory recursively',async  () => {
    fs.mkdirSync("test1")
    fs.mkdirSync("test1/test2")
    fs.writeFileSync("test1/test2/text.txt","this is test file.")
    await removeIgnorePermissionError("test1")
    expect(fs.existsSync("test1")).toBe(false);
  },{timeout:1000*1000});
});