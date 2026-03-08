import fs from 'node:fs/promises';
import path from 'node:path';

const frontendDir = process.cwd();
const standaloneDir = path.join(frontendDir, '.next', 'standalone');
const distDir = path.join(frontendDir, 'dist-standalone');

async function pathExists(targetPath) {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
}

async function findStandaloneServerEntry(rootDir, maxDepth = 4) {
  const rootEntry = path.join(rootDir, 'server.js');
  if (await pathExists(rootEntry)) {
    return rootEntry;
  }

  async function walk(currentDir, depth) {
    if (depth > maxDepth) {
      return null;
    }

    const entries = await fs.readdir(currentDir, {withFileTypes: true});
    const directories = entries
      .filter((entry) => entry.isDirectory())
      .map((entry) => entry.name)
      .sort();

    const files = entries
      .filter((entry) => entry.isFile() && entry.name === 'server.js')
      .map((entry) => path.join(currentDir, entry.name))
      .sort();

    if (files.length > 0) {
      return files[0];
    }

    for (const directory of directories) {
      const found = await walk(path.join(currentDir, directory), depth + 1);
      if (found) {
        return found;
      }
    }

    return null;
  }

  return walk(rootDir, 1);
}

async function copyOptionalDirectory(sourceDir, targetDir) {
  if (!(await pathExists(sourceDir))) {
    return;
  }

  await fs.mkdir(path.dirname(targetDir), {recursive: true});
  await fs.cp(sourceDir, targetDir, {recursive: true});
}

async function main() {
  if (!(await pathExists(standaloneDir))) {
    throw new Error('未找到 .next/standalone，请先执行 next build');
  }

  const standaloneEntry = await findStandaloneServerEntry(standaloneDir);
  if (!standaloneEntry) {
    throw new Error(
      "未找到 .next/standalone 下的 server.js，请确认 next.config.ts 已配置 output: 'standalone'",
    );
  }

  const entryRelativePath = path.relative(standaloneDir, standaloneEntry);
  const runtimeRelativeDir =
    path.dirname(entryRelativePath) === '.'
      ? ''
      : path.dirname(entryRelativePath);
  const runtimeDir = runtimeRelativeDir
    ? path.join(distDir, runtimeRelativeDir)
    : distDir;

  await fs.rm(distDir, {recursive: true, force: true});
  await fs.mkdir(distDir, {recursive: true});
  await fs.cp(standaloneDir, distDir, {recursive: true});

  await copyOptionalDirectory(
    path.join(frontendDir, '.next', 'static'),
    path.join(runtimeDir, '.next', 'static'),
  );
  await copyOptionalDirectory(
    path.join(frontendDir, 'public'),
    path.join(runtimeDir, 'public'),
  );

  console.log(`Packed standalone output: ${entryRelativePath}`);
  console.log(
    `Runtime directory: ${path.relative(frontendDir, runtimeDir) || '.'}`,
  );
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
});
