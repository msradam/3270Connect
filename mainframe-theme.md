# Yorkshire Mainframe Theme

A retro terminal-inspired design system with a Yorkshire twist, featuring a dark coal-black background with vibrant terminal green accents and CRT screen effects.

## Overview

This theme creates an authentic mainframe terminal aesthetic with modern web technologies, combining the nostalgia of legacy systems with contemporary design principles. The Yorkshire Mainframe Theme evokes the industrial heritage of West Yorkshire while providing a highly functional, accessible interface.

## Color Palette

### Core Colors

The theme uses the OKLCH color space for better perceptual uniformity and accessibility.

#### Background Colors
```css
--background: oklch(0.15 0.01 270);        /* Coal Black - Deep, rich black */
--card: oklch(0.18 0.02 270);              /* Slightly lighter coal for cards */
--popover: oklch(0.20 0.02 270);           /* Popover background */
--input: oklch(0.25 0.03 270);             /* Input field background */
--muted: oklch(0.25 0.02 270);             /* Muted elements */
```

#### Foreground Colors
```css
--foreground: oklch(0.75 0.15 145);        /* Terminal Green - Primary text */
--card-foreground: oklch(0.75 0.15 145);   /* Card text */
--popover-foreground: oklch(0.75 0.15 145); /* Popover text */
--muted-foreground: oklch(0.55 0.08 145);  /* Secondary text */
```

#### Accent Colors
```css
--primary: oklch(0.75 0.15 145);           /* Terminal Green - Primary actions */
--primary-foreground: oklch(0.15 0.01 270); /* Text on primary */
--accent: oklch(0.70 0.15 85);             /* Warning Amber - Highlights */
--accent-foreground: oklch(0.15 0.01 270); /* Text on accent */
--secondary: oklch(0.35 0.02 250);         /* Steel Gray - Secondary elements */
--secondary-foreground: oklch(0.75 0.15 145); /* Text on secondary */
```

#### Interactive Colors
```css
--border: oklch(0.30 0.05 145);            /* Border color with subtle green tint */
--ring: oklch(0.70 0.12 145);              /* Focus ring */
--destructive: oklch(0.60 0.20 15);        /* Error Red */
--destructive-foreground: oklch(0.95 0.02 145); /* Text on destructive */
```

### Color Usage Guidelines

- **Terminal Green (`--foreground`)**: Use for primary text, headings, and important information
- **Coal Black (`--background`)**: Primary background color, creates depth
- **Warning Amber (`--accent`)**: Use for highlights, interactive elements, and Yorkshire-themed content
- **Steel Gray (`--secondary`)**: Use for less prominent UI elements and borders
- **Error Red (`--destructive`)**: Reserved for error states and destructive actions

## Typography

### Font Families

```css
/* Terminal/Code Font */
.terminal-font {
  font-family: 'JetBrains Mono', monospace;
}

/* Content Font */
.content-font {
  font-family: 'Inter', sans-serif;
}
```

### Font Usage

- **JetBrains Mono**: Use for terminal output, system messages, status displays, and any text that should appear "computer-generated"
- **Inter**: Use for readable content, descriptions, and user-facing text

### Typography Best Practices

1. Use `terminal-font` class for:
   - Terminal command output
   - System status messages
   - Technical information
   - Headers with technical context

2. Use `content-font` class for:
   - Body text
   - Descriptions
   - User input areas
   - Accessible content sections

## Layout & Spacing

### Border Radius

The theme uses minimal, sharp corners to evoke the geometric precision of terminal interfaces:

```css
--radius: 0.125rem; /* Sharp, minimal rounding (2px) */
```

Apply minimal rounding to cards, inputs, and containers to maintain the technical aesthetic.

### Spacing Scale

Follows standard design system spacing, but keep layouts compact and information-dense to match the terminal aesthetic.

## Visual Effects

### CRT Screen Effect

Creates an authentic cathode ray tube screen appearance:

```css
.crt-screen {
  background:
    linear-gradient(180deg, rgba(0, 0, 0, 0.55), rgba(0, 0, 0, 0.75)),
    radial-gradient(ellipse at center, rgba(0, 255, 128, 0.08) 0%, transparent 65%),
    linear-gradient(transparent 0%, rgba(0, 255, 128, 0.04) 50%, transparent 100%),
    var(--card);
  position: relative;
}

.crt-screen::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: 
    repeating-linear-gradient(
      0deg,
      transparent,
      transparent 2px,
      rgba(0, 255, 128, 0.04) 2px,
      rgba(0, 255, 128, 0.04) 4px
    );
  pointer-events: none;
  z-index: 1;
}
```

**Usage**: Apply `.crt-screen` to the root container or main background elements to create the scanline effect.

### Terminal Cursor Animation

Blinking cursor effect for authentic terminal feel:

```css
@keyframes cursor-blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

.cursor {
  animation: cursor-blink 1s infinite;
}
```

**Usage**: Add `.cursor` class to any element that should represent an active cursor. Common character: `█`

### System Glow Effect

Adds a subtle phosphorescent glow to important elements:

```css
.system-glow {
  box-shadow: 
    0 0 10px rgba(0, 255, 128, 0.3),
    inset 0 0 10px rgba(0, 255, 128, 0.1);
}
```

**Usage**: Apply to important status indicators, active elements, or attention-drawing components.

### Yorkshire Accent Styling

Special styling for Yorkshire-themed content:

```css
.yorkshire-accent {
  color: oklch(0.70 0.15 85); /* Warning Amber */
  font-style: italic;
}

.yorkshire-accent:hover {
  text-shadow: 0 0 4px rgba(255, 193, 7, 0.3);
}
```

**Usage**: Apply to Yorkshire quotes, regional references, or flavor text.

## Component Patterns

### Terminal Window

```html
<div class="border border-border bg-card/30 rounded p-4">
  <div class="terminal-font text-primary">
    <!-- Terminal content -->
  </div>
</div>
```

### Status Display

```html
<div class="p-4 border border-border/50 rounded bg-card/30">
  <div class="text-xs terminal-font text-primary space-y-1">
    <div class="flex justify-between">
      <span>STATUS:</span>
      <span class="text-accent">ACTIVE</span>
    </div>
  </div>
</div>
```

### System Header

```html
<div class="border-b border-border bg-card/50">
  <div class="container mx-auto px-6 py-4">
    <h1 class="terminal-font text-xl text-primary font-bold">
      SYSTEM NAME
      <span class="cursor text-primary">█</span>
    </h1>
    <p class="text-sm text-muted-foreground mt-1">
      Status information
    </p>
  </div>
</div>
```

### Interactive Card with Glow

```html
<div class="system-glow p-4 border border-border rounded bg-card">
  <!-- Card content -->
</div>
```

## Tailwind CSS Integration

The theme integrates seamlessly with Tailwind CSS through custom CSS variables:

```css
@theme {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-destructive: var(--destructive);
  --color-destructive-foreground: var(--destructive-foreground);
  --color-border: var(--border);
  --color-input: var(--input);
  --color-ring: var(--ring);
}
```

### Using Tailwind Classes

```html
<!-- Background colors -->
<div class="bg-background">
<div class="bg-card">
<div class="bg-muted">

<!-- Text colors -->
<p class="text-foreground">
<p class="text-primary">
<p class="text-accent">
<p class="text-muted-foreground">

<!-- Border colors -->
<div class="border border-border">
<div class="border-primary">
```

## Application Examples

### Full Page Layout

```html
<div class="min-h-screen bg-background text-foreground crt-screen">
  <!-- Header -->
  <div class="border-b border-border bg-card/50">
    <div class="container mx-auto px-6 py-4">
      <h1 class="terminal-font text-xl text-primary font-bold">
        YORKSHIRE MAINFRAME SYSTEM
        <span class="cursor text-primary">█</span>
      </h1>
      <p class="text-sm text-muted-foreground mt-1">
        Secure Terminal Access • Hyperfocus Protocol Active
      </p>
    </div>
  </div>

  <!-- Main content -->
  <div class="container mx-auto px-6 py-8">
    <!-- Your content here -->
  </div>

  <!-- Footer -->
  <div class="mt-12 pt-8 border-t border-border">
    <div class="text-center text-xs text-muted-foreground terminal-font">
      <p>Yorkshire Mainframe Terminal • Authenticated Session</p>
      <p class="mt-1 yorkshire-accent">
        "Where there's muck, there's brass"
      </p>
    </div>
  </div>
</div>
```

### Terminal Command Output

```html
<div class="border border-border bg-card/30 rounded-sm p-4">
  <div class="terminal-font text-sm space-y-2">
    <div class="text-primary">$ system status</div>
    <div class="text-muted-foreground">
      Checking system components...
    </div>
    <div class="flex justify-between">
      <span class="text-foreground">NETWORK:</span>
      <span class="text-accent">SECURE</span>
    </div>
    <div class="flex justify-between">
      <span class="text-foreground">STATUS:</span>
      <span class="text-accent">OPERATIONAL</span>
    </div>
    <div class="text-primary">
      $ <span class="cursor">█</span>
    </div>
  </div>
</div>
```

### System Monitor Panel

```html
<div class="system-glow p-4 border border-border/50 rounded bg-card/30">
  <div class="text-xs terminal-font text-primary space-y-2">
    <div class="text-accent mb-2">SYSTEM MONITOR</div>
    <div class="flex justify-between">
      <span>CPU:</span>
      <span class="text-accent">45%</span>
    </div>
    <div class="flex justify-between">
      <span>MEMORY:</span>
      <span class="text-accent">62%</span>
    </div>
    <div class="flex justify-between">
      <span>DISK:</span>
      <span class="text-accent">78%</span>
    </div>
  </div>
</div>
```

## Accessibility Considerations

### Contrast Ratios

The theme maintains WCAG AA contrast ratios:
- Terminal Green on Coal Black: High contrast for excellent readability
- Warning Amber on Coal Black: Adequate contrast for secondary elements
- Muted text: Lower contrast for de-emphasized content (still meets AA for large text)

### Focus States

Always use the focus ring color for keyboard navigation:

```css
--ring: oklch(0.70 0.12 145); /* Visible terminal green focus indicator */
```

### Screen Readers

- Use semantic HTML
- Provide text alternatives for visual effects
- Ensure the CRT effect doesn't interfere with text readability

## Implementation Notes

### Dependencies

The theme requires:
- Tailwind CSS v4+ with custom CSS variable support
- OKLCH color space support (modern browsers)
- CSS Grid and Flexbox support

### Browser Support

- Chrome/Edge 111+
- Firefox 113+
- Safari 15.4+

### Performance

- The CRT scanline effect uses pseudo-elements to minimize DOM nodes
- Animations are GPU-accelerated where possible
- Use `will-change` sparingly for frequently animated elements

## Customization

### Adjusting Theme Colors

To customize the theme for different contexts, modify the CSS variables in `:root`:

```css
:root {
  /* Example: Warmer terminal green */
  --foreground: oklch(0.75 0.15 160);
  
  /* Example: Different accent color */
  --accent: oklch(0.70 0.15 60); /* More yellow */
}
```

### Adapting for Light Mode

For a light mode variant:

```css
:root.light-mode {
  --background: oklch(0.95 0.01 270);      /* Light gray */
  --foreground: oklch(0.25 0.15 145);      /* Dark green */
  --primary: oklch(0.35 0.20 145);         /* Darker green */
  --accent: oklch(0.50 0.15 85);           /* Darker amber */
}
```

### Regional Variations

The Yorkshire accent styling can be adapted for different regional themes by changing the accent color and hover effects while maintaining the overall mainframe aesthetic.

## Design Philosophy

The Yorkshire Mainframe Theme embodies:

1. **Industrial Heritage**: Coal-black backgrounds evoke Yorkshire's industrial past
2. **Technical Precision**: Sharp corners and monospace fonts reflect engineering exactness
3. **Functional Beauty**: Every visual element serves a purpose
4. **Nostalgic Authenticity**: CRT effects honor the legacy systems that built the digital age
5. **Modern Standards**: Built with contemporary web standards for performance and accessibility

## Credits & Inspiration

- Inspired by IBM 3270 terminals and mainframe systems
- Color palette influenced by vintage phosphor monitors
- Typography choices reflect classic terminal emulators
- Yorkshire cultural references celebrate regional pride

---

**Version**: 2.1.7  
**Last Updated**: 2025  
**Maintained by**: Yorkshire Developers  
**License**: MIT