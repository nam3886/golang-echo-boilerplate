# Pull & Refactor Pipeline

Sequential pipeline to transform Figma Make output into production-ready code.

## Step 1: Pull Code

```bash
# Clone/pull from Figma Make repo (from .figma-to-code.json → figma.makeRepo)
git clone https://github.com/{makeRepo}.git /tmp/figma-make-output
# Or if already cloned:
cd /tmp/figma-make-output && git pull
```

Copy relevant component files to a temp working directory for refactoring.

## Step 2: Analyze & Map

Read all component files from Figma Make output. Identify:
- Vietnamese component/file names
- Hardcoded hex colors and px values
- Monolith components (>100 lines or multiple variants)
- Random-ID SVG files
- Absolute positioning patterns

Create a rename map before modifying any files:
```
ComtDuAn           → ProjectComment
ToChucDanhSachDuAn → OrganizationProjectList
"THANH TUU"        → "ACHIEVEMENT"
"DU AN"            → "PROJECT"
svg-gc3e6obcro.ts  → icon-project.ts
```

## Step 3: Rename

1. Rename files to kebab-case English: `ComtDuAn.tsx` → `project-comment.tsx`
2. Rename component exports to PascalCase: `export function ProjectComment`
3. Update all import paths to match new filenames
4. Rename variant strings from Vietnamese to English

Claude reads Vietnamese natively — no translation API needed.

## Step 4: Split Monoliths

Threshold: split if component >100 lines OR has 2+ variant branches.

```
Before: ComtDuAn.tsx (250 lines, 4 variants)

After:
  project-comment/
    index.ts              → barrel export
    project-comment.tsx   → main component (variant router)
    variant-default.tsx   → default variant (<100 lines)
    variant-compact.tsx   → compact variant (<100 lines)
    types.ts              → shared types/interfaces
```

Rules:
- Each output file < 200 lines
- Shared props/types in `types.ts`
- Main component imports variants and routes via props
- Barrel export for clean imports

## Step 5: Design System Mapping

Read project's design tokens from `.figma-to-code.json` → `designSystem` paths.

| Figma Make Output | Replace With |
|-------------------|-------------|
| `#FF5722` (hardcoded hex) | `var(--color-primary)` or token reference |
| `16px`, `24px` (hardcoded) | Tailwind spacing: `p-4`, `p-6` |
| `position: absolute; top: 10px` | `flex`, `grid` utilities |
| `font-size: 14px` | `text-sm` (Tailwind) |
| `width: 375px` (fixed) | `w-full max-w-sm` (responsive) |

If no matching token exists, keep hardcoded value with `// TODO: map to design token` comment.

## Step 6: SVG Cleanup

1. Rename random-ID files: `svg-gc3e6obcro.ts` → `icon-project.ts`
2. Move to project's icons directory (from `paths.components` + `/icons/`)
3. If project uses icon components, convert to React component pattern:
   ```tsx
   export const IconProject = (props: SVGProps<SVGSVGElement>) => (
     <svg {...props}>...</svg>
   );
   ```

## Step 7: Copy to Target

```bash
# Move refactored files to --target path
cp -r /tmp/refactored/* {--target}/

# Verify TypeScript compiles
npx tsc --noEmit

# Verify imports resolve
# Check for broken import paths in output
```

## Post-Refactor Checklist

After each step, verify:
- [ ] No Vietnamese names remain in filenames, exports, or strings
- [ ] All files < 200 lines
- [ ] TypeScript compiles (`npx tsc --noEmit`)
- [ ] Import paths resolve correctly
- [ ] Kebab-case filenames throughout
- [ ] Hardcoded colors/spacing mapped where tokens exist
- [ ] Absolute positioning replaced with flex/grid where possible
- [ ] SVG files have meaningful names
