// @uncli/client: UNCLI's client without a UI (decision 0026). The types of
// the API, the Backend interface, the HTTP backend that reaches a UNCLI
// over its remote access server, and server-qualified session ids. The
// mock backend is @uncli/client/mock.
export type * from './types'
export { httpBackend, parseSSE, RemoteOnlyError, RPCError, type HttpOptions, type SSEFrame } from './http'
export { LOCAL, parseRef, qualify, serverKey, SESSION_FIRST, withServer, type SessionRef } from './identity'
export { dayKey, startOfDay } from './days'
export { SECTION_KINDS, type SectionKind } from './sections'
export { CLIENT_TEXT, setClientText, type ClientTextKey, type Translate } from './text'
