type Theme = 'dark' | 'light';

const KEY = 'yari.theme';

function readStored(): Theme | null {
  try {
    const v = localStorage.getItem(KEY);
    return v === 'light' || v === 'dark' ? v : null;
  } catch {
    return null;
  }
}

/** Dark by default. A light theme toggle is optional. The preference is not a secret. */
class ThemeState {
  current = $state<Theme>('dark');

  init() {
    this.apply(readStored() ?? 'dark');
  }

  toggle() {
    this.apply(this.current === 'dark' ? 'light' : 'dark');
    try {
      localStorage.setItem(KEY, this.current);
    } catch {
      // Storage may be blocked. The theme still applies for this run.
    }
  }

  private apply(theme: Theme) {
    this.current = theme;
    document.documentElement.classList.toggle('dark', theme === 'dark');
  }
}

export const theme = new ThemeState();
