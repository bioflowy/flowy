import { useState } from 'react'
import { Copy, Check } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface WDLSourceViewerProps {
  content: string
}

export function WDLSourceViewer({ content }: WDLSourceViewerProps) {
  const [copied, setCopied] = useState(false)

  async function handleCopy() {
    await navigator.clipboard.writeText(content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const lines = content.split('\n')

  return (
    <div className="relative rounded-md border bg-muted font-mono text-sm overflow-auto max-h-[500px]">
      <div className="sticky top-0 right-0 flex justify-end p-1 bg-muted/80">
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={handleCopy}
          aria-label="Copy code"
        >
          {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
        </Button>
      </div>
      <table className="w-full border-separate border-spacing-0">
        <tbody>
          {lines.map((line, i) => (
            <tr key={i}>
              <td className="select-none text-right pr-4 pl-4 text-muted-foreground w-[3rem] text-xs">
                {i + 1}
              </td>
              <td className="pr-4 whitespace-pre">{line || ' '}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
