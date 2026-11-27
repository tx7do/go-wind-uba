// Uba.Build.cs —— UE 插件模块构建：编译核心层（../../core/*.cpp）并桥接 FHttpModule。
using System;
using System.IO;
using UnrealBuildTool;

public class Uba : ModuleRules
{
	public Uba(ReadOnlyTargetRules Target) : base(Target)
	{
		PCHUsage = PCHUsageMode.UseExplicitOrSharedPCHs;

		PublicDependencyModuleNames.AddRange(new string[]
		{
			"Core"
		});

		PrivateDependencyModuleNames.AddRange(new string[]
		{
			"CoreUObject",
			"Engine",
			"HTTP",
			"Json"
		});

		// 纯 C++ 核心层源码（无 UE 依赖）：直接纳入模块编译
		var CoreDir = Path.Combine(ModuleDirectory, "..", "..", "..", "core");
		PublicIncludePaths.Add(CoreDir);
		PrivateSourceFiles.AddRange(Directory.EnumerateFiles(CoreDir, "*.cpp", SearchOption.TopDirectoryOnly));
	}
}
