import { loadDocument } from "./util/Internal"


async function test(): Promise<void>{
    const doc = await loadDocument(__dirname + '/data/examples/valid_scatter-wf3.cwl')
    console.log(doc)
}


const doc = test()
doc.then(() => {
    console.log('done')
}).catch((err) => { 
    console.log(err)
})