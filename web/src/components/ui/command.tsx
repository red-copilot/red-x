import { Command as CommandPrimitive } from 'cmdk'
import { Search } from 'lucide-react'
import { cn } from '@/lib/utils'

export const Command = CommandPrimitive
export function CommandInput({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Input>) {
  return (
    <div className="flex items-center gap-2 border-b px-3">
      <Search className="size-4 text-muted-foreground" />
      <CommandPrimitive.Input
        className={cn('h-11 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground', className)}
        {...props}
      />
    </div>
  )
}
export function CommandList({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.List>) {
  return <CommandPrimitive.List className={cn('max-h-[420px] overflow-auto p-2', className)} {...props} />
}
export function CommandEmpty(props: React.ComponentProps<typeof CommandPrimitive.Empty>) {
  return <CommandPrimitive.Empty className="py-8 text-center text-xs text-muted-foreground" {...props} />
}
export function CommandGroup({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Group>) {
  return <CommandPrimitive.Group className={cn('py-1 text-foreground', className)} {...props} />
}
export function CommandItem({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Item>) {
  return (
    <CommandPrimitive.Item
      className={cn(
        'flex cursor-default select-none items-center gap-2 rounded px-2.5 py-2 text-xs outline-none data-[selected=true]:bg-muted',
        className,
      )}
      {...props}
    />
  )
}
