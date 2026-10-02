/** Last-line filter so saved or streamed text cannot show tool protocol. */
export function displayChatText(content: string): string {
  let out = content
  out = out.replace(/<\|(?:im_start|im_end|eot_id|endoftext|end|start_header_id|end_header_id)\|>/gi, '')
  out = out.replace(/\[\/?INST\]/g, '')
  out = out.replace(/<tool_call>[\s\S]*?(?:<\/tool_call>|$)/gi, '')
  out = out.replace(/<tool_result>[\s\S]*?(?:<\/tool_result>|$)/gi, '')
  out = out.replace(/function_call\s*\([\s\S]*?\)/gi, '')
  for (let n = 0; n < 6; n += 1) {
    const key = protocolKey(out)
    if (key < 0) break
    const start = out.lastIndexOf('{', key)
    if (start < 0) {
      out = `${out.slice(0, key)} ${out.slice(key + 1)}`
      continue
    }
    const end = jsonObjectEnd(out, start)
    if (end < 0) {
      out = out.slice(0, start)
      break
    }
    out = `${out.slice(0, start)} ${out.slice(end)}`
  }
  const cleaned = out
    .replace(/```(?:json|JSON)?\s*```/g, '')
    .replace(/[ \t]+\n/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .replace(/(\S)[ \t]{2,}/g, '$1 ')
    .trim()
  return cleaned
}

/** Turn inline " - [title](url)" runs into Markdown lists the model forgot to break. */
export function prepareChatMarkdown(content: string): string {
  return displayChatText(content).replace(/ ([•*-]) (?=\[)/g, '\n$1 ')
}

function protocolKey(value: string): number {
  const marks = ['"tool_call"', '"tool_calls"', '"tool_result"']
  let found = -1
  for (const mark of marks) {
    const index = value.indexOf(mark)
    if (index >= 0 && (found < 0 || index < found)) found = index
  }
  return found
}

function jsonObjectEnd(value: string, start: number): number {
  let depth = 0
  let inString = false
  let escaped = false
  for (let i = start; i < value.length; i += 1) {
    const c = value[i]
    if (inString) {
      if (escaped) {
        escaped = false
        continue
      }
      if (c === '\\') {
        escaped = true
        continue
      }
      if (c === '"') inString = false
      continue
    }
    if (c === '"') inString = true
    else if (c === '{') depth += 1
    else if (c === '}') {
      depth -= 1
      if (depth === 0) return i + 1
    }
  }
  return -1
}
