
export interface HasId{
    getId():string
}

export class Dictionary<K extends HasId,V>{
    private dict:{[key:string]:V} = {}
    add(key:K,obj:V){
        this.dict[key.getId()] = obj
    }
    get(key:K): V | undefined{
        return this.dict[key.getId()]
    }
    remove(key:K){
        delete this.dict[key.getId()]
    }
    clear(){
        this.dict = {}
    }
}