using System;
using System.Collections.Generic;
using System.Globalization;
using System.Text;

namespace Uba
{
    internal static class Utils
    {
        private static readonly Random _rng = new Random();
        private static readonly object _rngLock = new object();

        /// <summary>生成 UUID（Guid）</summary>
        public static string NewUuid() => Guid.NewGuid().ToString("D", CultureInfo.InvariantCulture);

        /// <summary>生成 RFC3339 时间字符串（UTC），如 2026-06-28T08:30:00.000Z</summary>
        public static string ToRFC3339(DateTimeOffset? t = null)
        {
            var time = t ?? DateTimeOffset.UtcNow;
            // "O" round-trip 格式对 UTC 会输出 2026-06-28T08:30:00.0000000+00:00
            // protojson 期望 ...Z，这里手动格式化为毫秒精度 + Z
            return time.UtcDateTime.ToString("yyyy-MM-dd'T'HH:mm:ss.fff'Z'", CultureInfo.InvariantCulture);
        }

        /// <summary>浅合并字典（后者覆盖前者），返回新字典</summary>
        public static Dictionary<string, string> MergeProperties(
            params Dictionary<string, string>?[] sources)
        {
            var result = new Dictionary<string, string>();
            foreach (var src in sources)
            {
                if (src == null) continue;
                foreach (var kv in src)
                {
                    result[kv.Key] = kv.Value;
                }
            }
            return result;
        }

        /// <summary>按字符（rune）数限制长度，避免切断多字节字符。已 trim。</summary>
        public static string TrimAndLimit(string? s, int max)
        {
            if (string.IsNullOrEmpty(s)) return string.Empty;
            string t = s.Trim();
            // StringInfo 按 text element 计数，正确处理代理对/组合字符
            var info = new StringInfo(t);
            if (info.LengthInTextElements > max)
            {
                return info.SubstringByTextElements(0, max);
            }
            return t;
        }

        /// <summary>线程安全的简单随机字符串（用于回退 ID 等）</summary>
        public static string RandomString(int length)
        {
            const string chars = "abcdefghijklmnopqrstuvwxyz0123456789";
            lock (_rngLock)
            {
                var sb = new StringBuilder(length);
                for (int i = 0; i < length; i++)
                    sb.Append(chars[_rng.Next(chars.Length)]);
                return sb.ToString();
            }
        }
    }
}
