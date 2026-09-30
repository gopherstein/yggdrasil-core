import { TRAINING_EXPLAINER } from './display'

/** The two ideas the Train page teaches, side by side and visually distinct. */
export function ConceptCards({ compact = false }: { compact?: boolean }) {
  return (
    <div className="space-y-3">
      <div className="grid gap-3 md:grid-cols-2">
        <div className="card-outline space-y-2 border-l-4 !border-l-primary p-4">
          <p className="label-caps text-primary">Training</p>
          <h3 className="font-display text-lg font-semibold text-ink">Train how your AI behaves</h3>
          {!compact && (
            <p className="text-sm text-ink-muted">
              Examples of good answers teach its role, your terminology, the questions it should ask, the steps it
              follows, its tone, and how it formats replies.
            </p>
          )}
          <p className="text-xs text-ink-faint">Use: chat transcripts, question and answer pairs, sample replies.</p>
        </div>
        <div className="card-outline space-y-2 border-l-4 !border-l-mimir p-4">
          <p className="label-caps text-mimir">Knowledge</p>
          <h3 className="font-display text-lg font-semibold text-ink">Connect what your AI knows</h3>
          {!compact && (
            <p className="text-sm text-ink-muted">
              Information that changes or must stay exact is looked up when it is needed: inventory, prices, SKUs,
              catalogs, policies, and documents. Edit it any time without retraining.
            </p>
          )}
          <p className="text-xs text-ink-faint">Use: spreadsheets, CSV exports, product lists, policy documents.</p>
        </div>
      </div>
      <p className="text-sm text-ink-muted">{TRAINING_EXPLAINER}</p>
      <details className="text-sm text-ink-muted">
        <summary className="cursor-pointer text-ink">How it works</summary>
        <div className="mt-2 space-y-2">
          <p>
            <strong className="text-ink">LoRA</strong> trains a small add-on (an adapter) instead of changing the whole
            model. The original model stays untouched and available. <strong className="text-ink">QLoRA</strong> does
            the same on a compressed copy of the model so it fits in less memory.
          </p>
          <p>
            <strong className="text-ink">Connected knowledge</strong> is retrieval: before each answer Yggdrasil
            searches your sources for the passages that match the question and gives them to the model. This is often
            called RAG, retrieval-augmented generation.
          </p>
          <p>
            An <strong className="text-ink">epoch</strong> is one pass over all of your examples. Training loss measures
            how closely the model reproduces your examples. Lower loss does not by itself mean better answers, which is
            why you test before you deploy.
          </p>
        </div>
      </details>
    </div>
  )
}
