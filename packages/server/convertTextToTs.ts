import * as fs from 'fs';
import * as path from 'path';

const inputFilePath = process.argv[2];
const outputFilePath = process.argv[3];
if (!inputFilePath || !outputFilePath) {
  console.error('Usage: ts-node script.ts <inputFilePath> <outputFilePath>');
  process.exit(1);
}

try {
  // 指定されたファイルの内容を読み込む
  const fileContent = fs.readFileSync(inputFilePath, { encoding: 'utf8' });

  // TypeScriptのファイル内容を生成
  const jsonString = JSON.stringify(fileContent);
  const tsFileContent = `export const fileContent: string = ${jsonString};\n`;

  // 新しいTypeScriptファイルに内容を書き込む
  fs.writeFileSync(outputFilePath, tsFileContent, { encoding: 'utf8' });

  console.log(`File has been generated: ${path.resolve(outputFilePath)}`);
} catch (error) {
  console.error('Error reading or writing files:', error);
}
