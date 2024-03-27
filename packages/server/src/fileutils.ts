import * as fsp from 'node:fs/promises';
import * as fs from 'node:fs';
import * as path from 'node:path';
import { _logger } from './loghandler.js';

export async function removeIgnorePermissionError(file_path: string): Promise<void> {
    try {
      await deletePathRecursive(file_path);
    } catch (e) {
      if (e.code === 'EACCES' || e.code === 'EPERM') {
        _logger.info(`Permission denied when trying remove outdir ${file_path}`);
      } else {
        throw e
      }
    }
}
export async function copyRecursively(source, destination) {
  const stats = await fsp.stat(source);

  if (stats.isDirectory()) {
      await fsp.mkdir(destination, { recursive: true });
      const items = await fsp.readdir(source);

      for (let item of items) {
          const srcPath = path.join(source, item);
          const destPath = path.join(destination, item);

          await copyRecursively(srcPath, destPath);
      }
  } else if (stats.isFile()) {
    const stats = await fs.existsSync(destination);
    if(stats){
      await removeIgnorePermissionError(destination)
    }
    await fsp.copyFile(source, destination);
  }
}
async function deletePathRecursive(targetPath: string): Promise<void> {
  const stat = await fsp.lstat(targetPath);
  if (stat.isDirectory()) {
      // ディレクトリの場合、再帰的に削除
      const entries = await fsp.readdir(targetPath);
      await Promise.all(entries.map(async (entry) => {
          const fullPath = path.join(targetPath, entry);
          await deletePathRecursive(fullPath);
      }));
      await fsp.rmdir(targetPath);
  } else {
      // ファイルの場合、ファイルを削除
      await fsp.unlink(targetPath);
  }
}

export function isdir(dir_path: string) {
  return fs.existsSync(dir_path) && fs.lstatSync(dir_path).isDirectory();
}
export function isfile(file_path: string) {
  return fs.existsSync(file_path) && fs.lstatSync(file_path).isFile();
}
/**
 * Join multiple path together, similar to Python's os.path.join
 * If an absolute path is found, it discards the previous paths
 * @param paths paths to join.
 * @returns joined path
 */
export function pathJoin(...paths: string[]): string {
  let finalPath = '';
  for (const p of paths) {
    if (path.isAbsolute(p)) {
      finalPath = p;
    } else {
      finalPath = path.join(finalPath, p);
    }
  }
  return finalPath;
}
