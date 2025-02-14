
type FlowyResourceType = "tool" | "job"
export class FlowyToolURL {
    constructor(private id:string) {
    }
    toString():string{
        return "flowy://tool/"+this.id
    }
    getId():string{
        return this.id
    }
    toJSON():string{
        return this.toString()
    }
}
export class FlowyJobURL {
    constructor(private id:string) {
    }
    toString():string{
        return "flowy://job/"+this.id
    }
    getId():string{
        return this.id
    }
    toJSON():string{
        return this.toString()
    }
}
export function createFlowyToolURL(urlstring:string):FlowyToolURL{
    const [type,id] = checkUrl(urlstring)
    if(type !== "tool"){
        throw new Error("Invalid Flowy URI +" + urlstring)
    }
    return new FlowyToolURL(id)
}
export function createFlowyJobURL(urlstring:string):FlowyJobURL{
    const [type,id] = checkUrl(urlstring)
    if(type !== "job"){
        throw new Error("Invalid Flowy URI +" + urlstring)
    }
    return new FlowyJobURL(id)
}
function isFlowyResourceType(type: string): type is FlowyResourceType {
    return type === "tool" || type === "job"
}
function checkUrl(url: string): [FlowyResourceType,string] {
    const urlParts = url.split("/")
    if(!(urlParts[0] === "flowy:")){    
        throw new Error("Invalid Flowy URI +" + url)
    }
    if(!isFlowyResourceType(urlParts[2])){
        throw new Error("Invalid Flowy URI +" + url)
    }
    if(urlParts.length < 4){
        throw new Error("Invalid Flowy URI +" + url)
    }
    return [urlParts[2],urlParts[3]];
}

