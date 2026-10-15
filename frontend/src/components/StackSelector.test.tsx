// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../theme';
import { StackSelector } from './StackSelector';
import { STANDALONE_STACK, type StackSummary } from '../utils/stack';

afterEach(() => cleanup());

const stacks: StackSummary[] = [
  { name: 'blog', running: 1, total: 2 },
  { name: 'shop', running: 3, total: 3 },
  { name: STANDALONE_STACK, running: 0, total: 1 },
];

function renderSelector(props: Partial<React.ComponentProps<typeof StackSelector>> = {}) {
  const onSelect = vi.fn();
  render(
    <ThemeProvider>
      <StackSelector stacks={stacks} selected={null} onSelect={onSelect} {...props} />
    </ThemeProvider>,
  );
  return onSelect;
}

describe('StackSelector', () => {
  it('shows "All stacks" when nothing is selected', () => {
    renderSelector();
    expect(screen.getByRole('button', { name: 'Stack' }).textContent).toContain('All stacks');
  });

  it('lists every stack with running/total counts, plus All and Standalone', () => {
    renderSelector();
    fireEvent.click(screen.getByRole('button', { name: 'Stack' }));
    const options = screen.getAllByRole('option');
    expect(options.map((o) => o.textContent)).toEqual([
      'All stacks4/6',
      'blog1/2',
      'shop3/3',
      'Standalone0/1',
    ]);
  });

  it('selects a stack and closes', () => {
    const onSelect = renderSelector();
    fireEvent.click(screen.getByRole('button', { name: 'Stack' }));
    fireEvent.click(screen.getByText('shop'));
    expect(onSelect).toHaveBeenCalledWith('shop');
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('selects All as null and Standalone as its scope key', () => {
    const onSelect = renderSelector({ selected: 'shop' });
    fireEvent.click(screen.getByRole('button', { name: 'Stack' }));
    fireEvent.click(screen.getByText('All stacks'));
    expect(onSelect).toHaveBeenLastCalledWith(null);
    fireEvent.click(screen.getByRole('button', { name: 'Stack' }));
    fireEvent.click(screen.getByText('Standalone'));
    expect(onSelect).toHaveBeenLastCalledWith(STANDALONE_STACK);
  });

  it('marks the selected stack and shows it on the trigger', () => {
    renderSelector({ selected: 'blog' });
    const trigger = screen.getByRole('button', { name: 'Stack' });
    expect(trigger.textContent).toContain('blog');
    fireEvent.click(trigger);
    expect(screen.getByRole('option', { selected: true }).textContent).toContain('blog');
  });

  it('keeps a URL-selected stack that is not (yet) in the graph', () => {
    renderSelector({ selected: 'ghost' });
    expect(screen.getByRole('button', { name: 'Stack' }).textContent).toContain('ghost');
  });

  it('renders nothing when there are only standalone workloads', () => {
    const { container } = render(
      <ThemeProvider>
        <StackSelector stacks={[{ name: STANDALONE_STACK, running: 1, total: 1 }]} selected={null} onSelect={vi.fn()} />
      </ThemeProvider>,
    );
    expect(container.innerHTML).toBe('');
  });

  describe('keyboard', () => {
    // Options in order: All stacks, blog, shop, Standalone.
    const highlighted = () => {
      const list = screen.getByRole('listbox');
      const id = list.getAttribute('aria-activedescendant');
      return id ? document.getElementById(id)?.textContent : undefined;
    };

    it('opens with Arrow Down, focuses the list and highlights the selected stack', () => {
      renderSelector({ selected: 'shop' });
      fireEvent.keyDown(screen.getByRole('button', { name: 'Stack' }), { key: 'ArrowDown' });
      const list = screen.getByRole('listbox');
      expect(document.activeElement).toBe(list);
      expect(highlighted()).toContain('shop');
    });

    it('moves with the arrow keys and selects with Enter, returning focus to the trigger', () => {
      const onSelect = renderSelector();
      const trigger = screen.getByRole('button', { name: 'Stack' });
      fireEvent.keyDown(trigger, { key: 'ArrowDown' });
      const list = screen.getByRole('listbox');
      fireEvent.keyDown(list, { key: 'ArrowDown' });
      fireEvent.keyDown(list, { key: 'ArrowDown' });
      fireEvent.keyDown(list, { key: 'ArrowUp' });
      expect(highlighted()).toContain('blog');
      fireEvent.keyDown(list, { key: 'Enter' });
      expect(onSelect).toHaveBeenCalledWith('blog');
      expect(screen.queryByRole('listbox')).toBeNull();
      expect(document.activeElement).toBe(trigger);
    });

    it('jumps with Home and End, stays in bounds, and selects with Space', () => {
      const onSelect = renderSelector({ selected: 'blog' });
      fireEvent.keyDown(screen.getByRole('button', { name: 'Stack' }), { key: 'ArrowDown' });
      const list = screen.getByRole('listbox');
      fireEvent.keyDown(list, { key: 'End' });
      fireEvent.keyDown(list, { key: 'ArrowDown' }); // already last
      expect(highlighted()).toContain('Standalone');
      fireEvent.keyDown(list, { key: 'Home' });
      fireEvent.keyDown(list, { key: 'ArrowUp' }); // already first
      expect(highlighted()).toContain('All stacks');
      fireEvent.keyDown(list, { key: ' ' });
      expect(onSelect).toHaveBeenCalledWith(null);
    });

    it('closes with Escape without selecting and refocuses the trigger', () => {
      const onSelect = renderSelector();
      const trigger = screen.getByRole('button', { name: 'Stack' });
      fireEvent.keyDown(trigger, { key: 'ArrowDown' });
      fireEvent.keyDown(screen.getByRole('listbox'), { key: 'Escape' });
      expect(screen.queryByRole('listbox')).toBeNull();
      expect(onSelect).not.toHaveBeenCalled();
      expect(document.activeElement).toBe(trigger);
    });

    it('closes on Tab without selecting', () => {
      const onSelect = renderSelector();
      fireEvent.keyDown(screen.getByRole('button', { name: 'Stack' }), { key: 'ArrowDown' });
      fireEvent.keyDown(screen.getByRole('listbox'), { key: 'Tab' });
      expect(screen.queryByRole('listbox')).toBeNull();
      expect(onSelect).not.toHaveBeenCalled();
    });

    it('opening by click also puts focus in the list', () => {
      renderSelector();
      fireEvent.click(screen.getByRole('button', { name: 'Stack' }));
      expect(document.activeElement).toBe(screen.getByRole('listbox'));
    });
  });
});
