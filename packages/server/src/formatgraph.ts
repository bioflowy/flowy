import * as fs from 'node:fs/promises';
import * as path from 'node:path';
import * as url from 'node:url';
import * as n3 from 'n3';
import {RdfXmlParser} from "rdfxml-streaming-parser";
import { getFileContentFromS3 } from './builder.js';
import { filePathToURI } from './utils.js';
import { getManager } from './server/manager.js';
import { _logger } from './loghandler.js';

function guessContentType(path: string): string {
  const vals = path.split('.');
  const ext = vals[vals.length - 1];
  switch (ext) {
    case 'ttl':
      return 'text/turtle';
    default:
      return 'application/rdf+xml';
  }
}
export class FormatGraph {
  ontologyPaths: string[] = [];
  store: n3.Store = new n3.Store();
  constructor(ontologyPath?: string) {
    if (ontologyPath) {
      this.ontologyPaths.push(ontologyPath);
    } else {
      const dirname = path.dirname(url.fileURLToPath(import.meta.url));
      this.ontologyPaths.push(path.join(dirname, '../resources/EDAM_1.25.owl'));
    }
  }
  async loads(): Promise<void> {
    for (const ontologyPath of this.ontologyPaths) {
      const store = await this.load(ontologyPath);
      const all = store.getQuads(null,null,null,null)
      this.store.addQuads(all)
    }
    this.ontologyPaths.length = 0;
  }
  isLoaded() {
    return this.ontologyPaths.length === 0;
  }
  addOntology(ontologyPath) {
    this.ontologyPaths.push(ontologyPath);
  }
  async loadByN3(owlString:string):Promise<n3.Store> {
    const store = new n3.Store();
    return new Promise((resolve, reject) => {
      const parser = new n3.Parser()
      parser.parse(owlString,(err,quad,prefixes)=>{
        if(prefixes){
          _logger.info(`prefixes=${JSON.stringify(prefixes)}`)
        }
        if(err){
          reject(err)
        }else if(quad == null){
          resolve(store)
        }else{
          store.addQuad(quad)
        }
      })
    });
  }
  async loadRDFXMLStreamParser(owlString:string):Promise<n3.Store> {
    const store = new n3.Store();
    return new Promise((resolve, reject) => {
      const myParser = new RdfXmlParser();

      myParser.on('data',(e)=>{
        store.addQuad(e)
      })
      myParser.on('error',(e)=>{
        reject(e)
      })
      myParser.on('end',()=>{
        resolve(store)
      })
      myParser.write(owlString)
      myParser.end()
    });
  }
  async load(ontologyPath: string): Promise<n3.Store> {
    const store = new n3.Store();
    let owlData = '';
    if (ontologyPath.startsWith('s3:/')) {
      const config = getManager().getServerConfig();
      owlData = await getFileContentFromS3(config.sharedFileSystem, ontologyPath, true);
    } else {
      owlData = (await fs.readFile(ontologyPath)).toString('utf-8');
    }
    try{
      return await this.loadByN3(owlData)
    }catch(e){
      return await this.loadRDFXMLStreamParser(owlData)
    }
  }
  
  subClassOfPredicate = new n3.NamedNode('http://www.w3.org/2000/01/rdf-schema#subClassOf');
  equivalentClassPredicate = new n3.NamedNode('http://www.w3.org/2002/07/owl#equivalentClass');
  isSubClassOf(child: string, parent: string): boolean {
    if (child === parent) {
      return true;
    }
    if (!(child.startsWith('http:') && parent.startsWith('http:'))) {
      return false;
    }
    const childTerm = new n3.NamedNode(child);
    const parentTerm = new n3.NamedNode(parent);
    for (const result of this.store.match(childTerm, this.subClassOfPredicate)) {
      if (this.isSubClassOf(result.object.value, parent)) {
        return true;
      }
    }
    for (const result of this.store.match(childTerm, this.equivalentClassPredicate)) {
      if (this.isSubClassOf(result.object.value, parent)) {
        return true;
      }
    }
    for (const result of this.store.match(parentTerm, this.equivalentClassPredicate)) {
      if (this.isSubClassOf(child, result.object.value)) {
        return true;
      }
    }
    return false;
  }
}
