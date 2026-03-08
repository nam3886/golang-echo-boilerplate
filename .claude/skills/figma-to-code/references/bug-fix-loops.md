# Bug Fix Loops

Handle UI bugs by re-entering Figma Make with a fix prompt, then re-refactoring.

## UI Bug Fix: `--fix-ui "description"`

Re-enters Figma Make (existing project) + refactor pipeline.

```
1. Open existing Figma Make project
   -> Use .figma-to-code.json -> figma.makeUrl (or --make-url)

2. Send fix prompt to Figma Make
   -> "Fix: [bug description]. The current issue is: [details]"
   -> See figma-make-automation.md -> Existing Project Flow

3. Wait for generation + Save + Push to GitHub

4. Pull + Incremental refactor
   -> See refactor-strategy.md (only re-refactor changed files)

5. Copy updated components to --target
   -> Verify TypeScript compiles
```

## Composing Fix Prompts from Feedback

| Feedback | Figma Make Prompt |
|----------|-------------------|
| "Button color wrong" | "Fix: Change the button background color to match the original design" |
| "Layout broken on mobile" | "Fix: Make the layout responsive. On mobile, stack elements vertically" |
| "Missing icon" | "Fix: Add the missing [icon-name] icon next to the [element]" |
| "Spacing too tight" | "Fix: Increase spacing between [element A] and [element B]" |

If feedback includes a screenshot, describe the visual issue in the prompt.

## Iteration Limits

- Max 3 fix iterations per issue
- If not resolved after 3 attempts: escalate to human
- Report what was tried and why it didn't work
