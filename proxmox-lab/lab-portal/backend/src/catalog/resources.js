import { existsSync, statSync } from "node:fs";
import { dirname, extname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../../..");
const booksRoot = resolve(repositoryRoot, "books");

const bookResources = [
  {
    id: "mastering-linux-security-hardening-pdf",
    title: "Mastering Linux Security and Hardening",
    format: "PDF",
    platform: "Linux",
    fileName: "Donald A. Tevault - Mastering Linux Security And Hardening.pdf"
  },
  {
    id: "mastering-linux-security-hardening-epub",
    title: "Mastering Linux Security and Hardening",
    format: "EPUB",
    platform: "Linux",
    fileName: "Mastering Linux Security and Hardening_ A practical guide to -- Tevault, Donald A_ -- expert insight, 3, 2023.epub"
  },
  {
    id: "mastering-windows-security-hardening-pdf",
    title: "Mastering Windows Security and Hardening",
    format: "PDF",
    platform: "Windows",
    fileName: "Mastering Windows Security and Hardening_ Secure and protect -- Mark Dunkerley, Matt Tumbarello-2do.pdf"
  },
  {
    id: "mastering-windows-security-hardening-epub",
    title: "Mastering Windows Security and Hardening",
    format: "EPUB",
    platform: "Windows",
    fileName: "Mastering Windows Security and Hardening_ Secure and protect -- Mark Dunkerley, Matt Tumbarello-2nd.epub"
  }
];

function resourceFilePath(resource) {
  return resolve(booksRoot, resource.fileName);
}

function fileSizeBytes(filePath) {
  if (!existsSync(filePath)) return null;
  return statSync(filePath).size;
}

function publicResource(resource) {
  const filePath = resourceFilePath(resource);
  return {
    id: resource.id,
    title: resource.title,
    format: resource.format,
    platform: resource.platform,
    extension: extname(resource.fileName).replace(".", "").toUpperCase(),
    fileName: resource.fileName,
    sizeBytes: fileSizeBytes(filePath),
    available: existsSync(filePath)
  };
}

export function listDownloadableResources() {
  return bookResources.map(publicResource);
}

export function findDownloadableResource(resourceId) {
  const resource = bookResources.find((candidate) => candidate.id === resourceId);
  if (!resource) return null;

  const filePath = resourceFilePath(resource);
  return {
    ...publicResource(resource),
    filePath
  };
}
