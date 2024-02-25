
import { loadDocument, loadDocumentByString } from '../'
import fs from 'fs'
import url from 'url'

describe('Example Tests', () => {

    it('valid_scatter_wf3', async () => {
        const doc = await loadDocument(__dirname + '/data/examples/valid_scatter-wf3.cwl')
        console.log(doc)
    })
})
