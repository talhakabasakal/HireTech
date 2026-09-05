import { createHash } from "node:crypto";
import { readdir, readFile, writeFile } from "node:fs/promises";
import { relative, resolve } from "node:path";
import process from "node:process";
import ts from "typescript";

const projectRoot = resolve(import.meta.dirname, "..");
const sourceRoot = resolve(projectRoot, "core", "infrastructure");
const manifestPath = resolve(projectRoot, "config", "graphql-persisted-operations.json");

async function sourceFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(entries.map(async (entry) => {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) return sourceFiles(path);
    return entry.isFile() && path.endsWith(".ts") ? [path] : [];
  }));
  return files.flat().sort();
}

function stringBindings(sourceFile) {
  const bindings = new Map();
  for (const statement of sourceFile.statements) {
    if (!ts.isVariableStatement(statement) || !(statement.declarationList.flags & ts.NodeFlags.Const)) continue;
    for (const declaration of statement.declarationList.declarations) {
      if (ts.isIdentifier(declaration.name) && declaration.initializer) bindings.set(declaration.name.text, declaration.initializer);
    }
  }
  return bindings;
}

function evaluateString(node, bindings, resolving = new Set()) {
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return node.text;
  if (ts.isParenthesizedExpression(node)) return evaluateString(node.expression, bindings, resolving);
  if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.PlusToken) {
    return evaluateString(node.left, bindings, resolving) + evaluateString(node.right, bindings, resolving);
  }
  if (ts.isIdentifier(node)) {
    if (resolving.has(node.text)) throw new Error(`Circular string binding: ${node.text}`);
    const value = bindings.get(node.text);
    if (!value) throw new Error(`GraphQL document references a non-local string binding: ${node.text}`);
    return evaluateString(value, bindings, new Set([...resolving, node.text]));
  }
  if (ts.isTemplateExpression(node)) {
    let value = node.head.text;
    for (const span of node.templateSpans) value += evaluateString(span.expression, bindings, resolving) + span.literal.text;
    return value;
  }
  throw new Error(`GraphQL document is not statically evaluable (${ts.SyntaxKind[node.kind]})`);
}

async function collectOperations() {
  const operations = new Map();
  for (const file of await sourceFiles(sourceRoot)) {
    const source = await readFile(file, "utf8");
    const sourceFile = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
    const bindings = stringBindings(sourceFile);

    function visit(node) {
      if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "graphqlRequest") {
        if (!node.arguments[0]) throw new Error(`${relative(projectRoot, file)} has graphqlRequest without a document`);
        const document = evaluateString(node.arguments[0], bindings);
        const match = document.match(/^\s*(query|mutation|subscription)\s+([_A-Za-z][_0-9A-Za-z]*)\b/);
        if (!match) throw new Error(`${relative(projectRoot, file)} has an anonymous or invalid GraphQL document`);
        const [, type, name] = match;
        const previous = operations.get(name);
        if (previous && previous.query !== document) throw new Error(`GraphQL operation name ${name} is used by different documents`);
        operations.set(name, {
          name,
          type,
          sha256Hash: createHash("sha256").update(document, "utf8").digest("hex"),
          source: relative(projectRoot, file),
          query: document,
        });
      }
      ts.forEachChild(node, visit);
    }
    visit(sourceFile);
  }
  if (operations.size === 0) throw new Error("No GraphQL operations were found");
  return [...operations.values()].sort((a, b) => a.name.localeCompare(b.name));
}

const operations = await collectOperations();
const manifest = `${JSON.stringify({ version: 1, algorithm: "sha256", operations }, null, 2)}\n`;
const mode = process.argv[2] ?? "--write";

if (mode === "--check") {
  const existing = await readFile(manifestPath, "utf8").catch(() => "");
  if (existing !== manifest) {
    process.stderr.write("Persisted-operation manifest is missing or stale. Run npm run persisted-operations:generate.\n");
    process.exitCode = 1;
  } else {
    process.stdout.write(`Verified ${operations.length} persisted GraphQL operations.\n`);
  }
} else if (mode === "--env") {
  process.stdout.write(`${operations.map(({ sha256Hash }) => sha256Hash).join(",")}\n`);
} else if (mode === "--write") {
  await writeFile(manifestPath, manifest);
  process.stdout.write(`Wrote ${operations.length} operations to ${relative(projectRoot, manifestPath)}.\n`);
} else {
  throw new Error(`Unknown mode: ${mode}`);
}
