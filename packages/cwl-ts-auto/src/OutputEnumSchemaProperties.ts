
import * as Internal from './util/Internal.js'


/**
 * Auto-generated interface for https://w3id.org/cwl/cwl#OutputEnumSchema
 */
export interface OutputEnumSchemaProperties extends Internal.EnumSchemaProperties, Internal.OutputSchemaProperties {
                    
  extensionFields?: Internal.Dictionary<any>

  /**
   * The identifier for this type
   */
  name?: undefined | string

  /**
   * Defines the set of valid symbols.
   */
  symbols: Array<string>

  /**
   * Must be `enum`
   */
  type: Internal.enum_d961d79c225752b9fadb617367615ab176b47d77

  /**
   * A short, human-readable label of this object.
   */
  label?: undefined | string

  /**
   * A documentation string for this object, or an array of strings which should be concatenated.
   */
  doc?: undefined | string | Array<string>
}