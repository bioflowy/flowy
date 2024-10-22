import { get } from "node:http";

type FlowyResourceType = "tool" | "job" | "dataset";

export abstract class FlowyURL {
  abstract toString(): string;
  abstract getId(): string;
  abstract toJSON(): string;
  abstract getType(): FlowyResourceType;
}
export class FlowyToolURL extends FlowyURL {
  constructor(private id: string) {
    super();
  }
  toString(): string {
    return "flowy://tool/" + this.id;
  }
  getId(): string {
    return this.id.split("#")[0];
  }
  getFragment(): string {
    const flags = this.id.split("#");
    if (flags.length > 1) {
      return "#"+flags[1];
    }
    return ""
  }
  toJSON(): string {
    return this.toString();
  }
  getType(): FlowyResourceType {
    return "tool";
  }
}
export class FlowyDatasetURL extends FlowyURL {
  constructor(private id: string) {
    super();
  }
  getType(): FlowyResourceType {
    return "dataset";
  }
  toString(): string {
    return "flowy://dataset/" + this.id;
  }
  getId(): string {
    return this.id;
  }
  toJSON(): string {
    return this.toString();
  }
}
export class FlowyJobURL extends FlowyURL {
  constructor(private id: string) {
    super();
  }
  getType(): FlowyResourceType {
    return "job";
  }
  toString(): string {
    return "flowy://job/" + this.id;
  }
  getId(): string {
    return this.id;
  }
  toJSON(): string {
    return this.toString();
  }
}
export function createFlowyToolURL(urlstring: string): FlowyToolURL {
  const [type, id] = checkUrl(urlstring);
  if (type !== "tool") {
    throw new Error("Invalid Flowy URI +" + urlstring);
  }
  return new FlowyToolURL(id);
}
export function createFlowyDatasetURL(urlstring: string): FlowyDatasetURL {
  const [type, id] = checkUrl(urlstring);
  if (type !== "dataset") {
    throw new Error("Invalid Flowy URI +" + urlstring);
  }
  return new FlowyDatasetURL(id);
}
export function createFlowyJobURL(urlstring: string): FlowyJobURL {
  const [type, id] = checkUrl(urlstring);
  if (type !== "job") {
    throw new Error("Invalid Flowy URI +" + urlstring);
  }
  return new FlowyJobURL(id);
}
function isFlowyResourceType(type: string): type is FlowyResourceType {
  return type === "tool" || type === "job" || type === "dataset";
}
function checkUrl(url: string): [FlowyResourceType, string] {
  const urlParts = url.split("/");
  if (!(urlParts[0] === "flowy:")) {
    throw new Error("Invalid Flowy URI +" + url);
  }
  if (!isFlowyResourceType(urlParts[2])) {
    throw new Error("Invalid Flowy URI +" + url);
  }
  if (urlParts.length < 4) {
    throw new Error("Invalid Flowy URI +" + url);
  }
  return [urlParts[2], urlParts[3]];
}
