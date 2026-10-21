import { describe, it, expect, beforeAll } from 'vitest';
import i18n from './index';

beforeAll(async () => {
  if (!i18n.isInitialized) {
    await i18n.init();
  }
});

describe('i18n configuration', () => {
  it('resolves a known key to its Chinese string by default', () => {
    expect(i18n.t('common.back')).toBe('返回');
  });

  it('falls back to Chinese for an unsupported language', async () => {
    await i18n.changeLanguage('zz-ZZ');
    expect(i18n.resolvedLanguage).toBe('zh');
    expect(i18n.t('common.save')).toBe('保存');
  });

  it('switches to English properly', async () => {
    await i18n.changeLanguage('en');
    expect(i18n.t('common.back')).toBe('Back');
    expect(i18n.t('test.greeting', { name: 'Sam' })).toBe('Hi, Sam');
    expect(i18n.t('test.points', { count: 1 })).toBe('1 point');
    expect(i18n.t('test.points', { count: 3 })).toBe('3 points');
    await i18n.changeLanguage('zh');
  });

  it('interpolates variables in Chinese', () => {
    expect(i18n.t('test.greeting', { name: '小明' })).toBe('你好，小明');
  });
});
