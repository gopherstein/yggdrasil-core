import type { SpecializedAIView } from '@/types/api'
import type { StepID } from './display'

const notes: Record<StepID, string> = {
  describe:
    'This example is an assistant for a tire shop. Yggdrasil drafted the instructions from the description. Edit anything; nothing trains until you press Train.',
  base: 'Yggdrasil picked the smallest model that trains comfortably on this computer and allows business use. Smaller models train in minutes.',
  material:
    'Three files were added, and Yggdrasil classified each one. The chats became Training because they show how to answer. The inventory became Knowledge because prices and stock change. The policies became Knowledge because the assistant should quote them exactly. Open "See sample files" to read them.',
  examples:
    'Three examples are flawed on purpose. The duplicate and the question with no answer are left out automatically. The answer that states a price still trains, which is why it is flagged. Try excluding it.',
  plan: 'The example uses Quick, so training takes a few minutes. Everything under "Stays connected" is searched on each question instead of being trained in.',
  train:
    'Watch the training loss fall as the model learns the examples. A low loss means it copies them well, not that its answers are right. That is what the next step checks.',
  test: 'Compare the columns. The specialized answers should ask for the vehicle and sign off as Tread Right Tires. Look for made-up details too, such as a tire size that sounds right but is not: a small model learns style well and facts poorly, which is why prices and sizes belong in knowledge. Add a test prompt such as "How much is the Pilot Sport 4?" to see connected knowledge at work.',
  deploy:
    'After you deploy, ask it in Chat for the price of a tire. Then open Knowledge, edit the inventory, and ask again. The answer changes without retraining. Delete the example from the Describe step when you are done.',
}

export function ExampleNote({ view, step }: { view: SpecializedAIView; step: StepID }) {
  if (!view.example) return null
  return (
    <div className="rounded-lg border border-info/30 bg-info/10 p-3 text-sm text-ink">
      <p className="label-caps mb-1 text-info">In this example</p>
      <p className="text-ink-muted">{notes[step]}</p>
    </div>
  )
}
