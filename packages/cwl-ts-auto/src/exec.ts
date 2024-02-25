import { loadDocument } from "./util/Internal"


async function test(){
    const doc = await loadDocument( 'src/test/data/examples/valid_schemadef-wf.cwl')
    console.log(doc)
}


const doc = test()
doc.then(() => {
    console.log('done')
}).catch((err) => { 
    console.log(err)
})