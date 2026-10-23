import { render } from 'preact';
import { App } from './App';
import css from './styles.css?inline';

const style = document.createElement('style');
style.textContent = css;
document.head.append(style);

render(<App onClose={() => window.close()} />, document.getElementById('root')!);

// autoFocus alone is unreliable in popups opened from a keyboard shortcut.
window.addEventListener('load', () => document.querySelector<HTMLInputElement>('input')?.focus());
