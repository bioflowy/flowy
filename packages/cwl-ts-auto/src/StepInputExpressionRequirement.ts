
import {
  Dictionary,
  expandUrl,
  loadField,
  LoaderInstances,
  LoadingOptions,
  Saveable,
  ValidationException,
  prefixUrl,
  save,
  saveRelativeUri
} from './util/Internal.js'
import { v4 as uuidv4 } from 'uuid'
import * as Internal from './util/Internal.js'


/**
 * Auto-generated class implementation for https://w3id.org/cwl/cwl#StepInputExpressionRequirement
 *
 * Indicate that the workflow platform must support the `valueFrom` field
 * of [WorkflowStepInput](#WorkflowStepInput).
 * 
 */
export class StepInputExpressionRequirement extends Saveable implements Internal.StepInputExpressionRequirementProperties {
  extensionFields?: Internal.Dictionary<any>

  /**
   * Always 'StepInputExpressionRequirement'
   */
  class_: Internal.StepInputExpressionRequirement_class


  constructor ({loadingOptions, extensionFields, class_ = Internal.StepInputExpressionRequirement_class.STEPINPUTEXPRESSIONREQUIREMENT} : {loadingOptions?: LoadingOptions} & Internal.StepInputExpressionRequirementProperties) {
    super(loadingOptions)
    this.extensionFields = extensionFields ?? {}
    this.class_ = class_
  }

  /**
   * Used to construct instances of {@link StepInputExpressionRequirement }.
   *
   * @param __doc                           Document fragment to load this record object from.
   * @param baseuri                         Base URI to generate child document IDs against.
   * @param loadingOptions                  Context for loading URIs and populating objects.
   * @param docRoot                         ID at this position in the document (if available)
   * @returns                               An instance of {@link StepInputExpressionRequirement }
   * @throws {@link ValidationException}    If the document fragment is not a
   *                                        {@link Dictionary} or validation of fields fails.
   */
  static override async fromDoc (__doc: any, baseuri: string, loadingOptions: LoadingOptions,
    docRoot?: string): Promise<Saveable> {
    const _doc = Object.assign({}, __doc)
    const __errors: ValidationException[] = []
            
    let class_
    try {
      class_ = await loadField(_doc.class, LoaderInstances.uriStepInputExpressionRequirement_classLoaderFalseTrueNone,
        baseuri, loadingOptions)
    } catch (e) {
      if (e instanceof ValidationException) {
        __errors.push(
          new ValidationException('the `class` field is not valid because: ', [e])
        )
      } else {
        throw e
      }
    }

    const extensionFields: Dictionary<any> = {}
    for (const [key, value] of Object.entries(_doc)) {
      if (!StepInputExpressionRequirement.attr.has(key)) {
        if ((key as string).includes(':')) {
          const ex = expandUrl(key, '', loadingOptions, false, false)
          extensionFields[ex] = value
        } else {
          __errors.push(
            new ValidationException(`invalid field ${key as string}, \
            expected one of: \`class\``)
          )
          break
        }
      }
    }

    if (__errors.length > 0) {
      throw new ValidationException("Trying 'StepInputExpressionRequirement'", __errors)
    }

    const schema = new StepInputExpressionRequirement({
      extensionFields: extensionFields,
      loadingOptions: loadingOptions,
      class_: class_
    })
    return schema
  }
        
  save (top: boolean = false, baseUrl: string = '', relativeUris: boolean = true)
  : Dictionary<any> {
    const r: Dictionary<any> = {}
    for (const ef in this.extensionFields) {
      r[prefixUrl(ef, this.loadingOptions.vocab)] = this.extensionFields.ef
    }

    if (this.class_ != null) {
      const u = saveRelativeUri(this.class_, baseUrl, false,
                                relativeUris, undefined)
      if (u != null) {
        r.class = u
      }
    }
                
    if (top) {
      if (this.loadingOptions.namespaces != null) {
        r.$namespaces = this.loadingOptions.namespaces
      }
      if (this.loadingOptions.schemas != null) {
        r.$schemas = this.loadingOptions.schemas
      }
    }
    return r
  }
            
  static attr: Set<string> = new Set(['class'])
}
