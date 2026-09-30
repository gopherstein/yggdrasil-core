/** File types knowledge and training material accept. */
export const UPLOAD_ACCEPT = '.txt,.md,.markdown,.csv,.tsv,.json,.jsonl,.html,.htm,.xlsx,.pdf'

const BINARY = ['.xlsx', '.pdf']

export const MAX_UPLOAD_BYTES = 20 * 1024 * 1024

export interface Upload {
  filename: string
  /** Set for text files. */
  text?: string
  /** Set for binary files such as spreadsheets. */
  contentBase64?: string
}

export function isBinaryUpload(filename: string): boolean {
  const lower = filename.toLowerCase()
  return BINARY.some((ext) => lower.endsWith(ext))
}

export async function readUpload(file: File): Promise<Upload> {
  if (!isBinaryUpload(file.name)) {
    return { filename: file.name, text: await file.text() }
  }
  const bytes = new Uint8Array(await file.arrayBuffer())
  let binary = ''
  const chunk = 0x8000
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk))
  }
  return { filename: file.name, contentBase64: btoa(binary) }
}
