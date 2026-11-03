// The chat's API layer: @uncli/client (decision 0026), with the client's
// few messages in UNCLI's strings, in its language (or a host's).
import { setClientText } from '@uncli/client'
import { t } from '../i18n.svelte'

export * from '@uncli/client'

setClientText((key, params) => t(key, params))
