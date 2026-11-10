import { describe, expect, it } from 'vitest';
import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FileDrop } from './FileDrop';

const limits = { max_files: 2, max_file_mb: 1, max_total_mb: 2, allowed_extensions: ['png', 'pdf'], title_max: 140, body_max: 4000 };

function Harness() {
  const [files, setFiles] = useState<File[]>([]);
  return <FileDrop files={files} onChange={setFiles} limits={limits} />;
}

describe('FileDrop', () => {
  it('lists accepted files, explains rejected ones and removes files', async () => {
    const user = userEvent.setup({ applyAccept: false });
    const { container } = render(<Harness />);
    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(input, [new File(['a'], 'фото.png', { type: 'image/png' }), new File(['b'], 'virus.exe')]);

    expect(screen.getByText('фото.png')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('virus.exe: Такой формат не поддерживается');
    expect(screen.getByText(/сведения об устройстве/)).toBeInTheDocument(); // EXIF warning for photos

    await user.click(screen.getByRole('button', { name: 'Убрать файл фото.png' }));
    expect(screen.queryByText('фото.png')).not.toBeInTheDocument();
  });
});
