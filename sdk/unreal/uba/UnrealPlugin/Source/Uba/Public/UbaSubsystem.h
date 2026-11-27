// UbaSubsystem.h —— GoWind UBA Unreal 埋点子系统（Blueprint 可调用门面）。
// 核心层为纯 C++（../../core/uba_core.h）；本子系统负责：配置注入、FHttpModule 传输桥接、
// 引擎生命周期接前后台/退出 flush。
//
// 用法（C++）：
//   UUbaSubsystem* Uba = GEngine->GetEngineSubsystem<UUbaSubsystem>();
//   Uba->Init(UbaSettings{TEXT("app_id"), TEXT("app_secret"), TEXT("http://collector:5700")});
//   Uba->Track(TEXT("level_pass"), {{"level", "1-1"}}, 100 /*score*/);
// Blueprint：启用插件后在「Uba」分类下直接调用同名节点。
#pragma once

#include "CoreMinimal.h"
#include "Subsystems/EngineSubsystem.h"
#include "UbaSubsystem.generated.h"

USTRUCT(BlueprintType)
struct FUbaSettings
{
	GENERATED_BODY()

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "Uba")
	FString AppId;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "Uba")
	FString AppSecret;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "Uba")
	FString Endpoint = TEXT("http://localhost:5700");

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "Uba")
	FString AppVersion;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "Uba")
	FString Channel;

	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "Uba")
	bool Debug = false;
};

/// 埋点子系统：引擎级单例（GetEngineSubsystem），游戏内任意线程可 Track（内部锁）。
/// Flush 为同步语义（与其他端 SDK 一致）——勿在游戏线程高频调用，退出/后台时机使用。
UCLASS()
class UBA_API UUbaSubsystem : public UEngineSubsystem
{
	GENERATED_BODY()

public:
	/// 初始化（幂等；重复调用返回已有实例状态）。必须在 Track 前调用。
	UFUNCTION(BlueprintCallable, Category = "Uba")
	void Init(const FUbaSettings& Settings);

	UFUNCTION(BlueprintPure, Category = "Uba")
	bool IsInitialized() const;

	/// 上报自定义行为事件（Properties 键值均为字符串）。
	UFUNCTION(BlueprintCallable, Category = "Uba")
	void Track(const FString& EventName,
	           const TMap<FString, FString>& Properties,
	           const FString& EventCategory = TEXT(""),
	           const FString& ObjectType = TEXT(""),
	           const FString& ObjectId = TEXT(""),
	           int32 Score = 0,
	           const FString& Amount = TEXT(""),
	           const FString& ServerId = TEXT(""),
	           int32 Level = 0);

	/// 上报风险事件（进入 uba_risk_events 通道）。
	UFUNCTION(BlueprintCallable, Category = "Uba")
	void TrackRisk(const FString& EventName,
	               const FString& RiskType,
	               const FString& RiskLevel,
	               float RiskScore,
	               const FString& Description,
	               const TMap<FString, FString>& Evidence);

	/// 登录绑定用户 ID。
	UFUNCTION(BlueprintCallable, Category = "Uba")
	void Identify(int64 UserId);

	/// 登出解绑。
	UFUNCTION(BlueprintCallable, Category = "Uba")
	void ResetUser();

	/// 手动冲刷队列（关键节点立即上报 / 退后台时机调用）。
	UFUNCTION(BlueprintCallable, Category = "Uba")
	void Flush();

	/// 关闭并冲刷剩余队列（ApplicationWillDeactivate / 退出时机）。
	UFUNCTION(BlueprintCallable, Category = "Uba")
	void Shutdown();

private:
	virtual void Deinitialize() override;

	// 内部持纯 C++ 客户端（Pimpl，避免 UE 反射器接触核心头）
	TUniquePtr<class FUbaClientHolder> Holder;
};
