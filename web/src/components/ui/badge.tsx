import { cva, type VariantProps } from 'class-variance-authority'
import type { HTMLAttributes } from 'react'
import { cn } from '@/lib/utils'

const badgeVariants = cva('inline-flex items-center rounded-md border px-2 py-0.5 text-[11px] font-medium', {
  variants: {
    variant: {
      default: 'border-transparent bg-secondary text-secondary-foreground',
      outline: 'border-border bg-card text-muted-foreground',
      success: 'border-[#b7e4cd] bg-[#edf9f2] text-[#027a48]',
      warning: 'border-[#f1d69f] bg-[#fff8e6] text-[#8a5200]',
      danger: 'border-[#f1b8b2] bg-[#fff1ef] text-[#b42318]',
      info: 'border-[#bfd4ee] bg-[#eef5fd] text-[#2563a5]',
    },
  },
  defaultVariants: { variant: 'default' },
})

export function Badge({
  className,
  variant,
  ...props
}: HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />
}
