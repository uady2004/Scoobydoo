import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../data/datasources/wallet_remote_datasource.dart';
import '../../data/repositories/wallet_repository_impl.dart';
import '../../domain/entities/wallet_entity.dart';
import '../../domain/usecases/get_balance_usecase.dart';
import '../../../../core/usecases/usecase.dart';

// ---------------------------------------------------------------------------
// Infrastructure providers
// ---------------------------------------------------------------------------

final walletDataSourceProvider =
    Provider<WalletRemoteDataSource>((_) => WalletRemoteDataSourceImpl());

final walletRepositoryProvider = Provider<WalletRepositoryImpl>(
  (ref) => WalletRepositoryImpl(
    dataSource: ref.read(walletDataSourceProvider),
  ),
);

// ---------------------------------------------------------------------------
// walletProvider — AsyncNotifier<WalletEntity>
// ---------------------------------------------------------------------------

class WalletNotifier extends AsyncNotifier<WalletEntity> {
  @override
  Future<WalletEntity> build() async => _fetch();

  Future<WalletEntity> _fetch() async {
    final repo = ref.read(walletRepositoryProvider);
    final usecase = GetBalanceUseCase(repo);
    final result = await usecase(const NoParams());
    return result.fold(
      (failure) => throw Exception(failure.message),
      (entity) => entity,
    );
  }

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(_fetch);
  }

  /// Optimistically updates coin balance after a successful purchase.
  void creditCoins(int coins) {
    state.whenData((wallet) {
      state = AsyncData(
        WalletEntity(
          coinBalance: wallet.coinBalance + coins,
          diamondBalance: wallet.diamondBalance,
          updatedAt: DateTime.now(),
        ),
      );
    });
  }

  /// Optimistically deducts coins when sending a gift.
  bool deductCoins(int amount) {
    bool success = false;
    state.whenData((wallet) {
      if (wallet.coinBalance >= amount) {
        state = AsyncData(
          WalletEntity(
            coinBalance: wallet.coinBalance - amount,
            diamondBalance: wallet.diamondBalance,
            updatedAt: DateTime.now(),
          ),
        );
        success = true;
      }
    });
    return success;
  }
}

final walletProvider =
    AsyncNotifierProvider<WalletNotifier, WalletEntity>(WalletNotifier.new);

// ---------------------------------------------------------------------------
// coinPackagesProvider — FutureProvider<List<CoinPackageEntity>>
// ---------------------------------------------------------------------------

final coinPackagesProvider =
    FutureProvider<List<CoinPackageEntity>>((ref) async {
  final repo = ref.read(walletRepositoryProvider);
  final result = await repo.getCoinPackages();
  return result.fold(
    (failure) => throw Exception(failure.message),
    (packages) => packages,
  );
});

// ---------------------------------------------------------------------------
// transactionsProvider — AsyncNotifier with cursor-based pagination
// ---------------------------------------------------------------------------

class TransactionState {
  final List<TransactionEntity> items;
  final bool isLoading;
  final bool hasMore;
  final String? nextCursor;
  final String? error;

  const TransactionState({
    this.items = const [],
    this.isLoading = false,
    this.hasMore = true,
    this.nextCursor,
    this.error,
  });

  TransactionState copyWith({
    List<TransactionEntity>? items,
    bool? isLoading,
    bool? hasMore,
    String? nextCursor,
    String? error,
  }) {
    return TransactionState(
      items: items ?? this.items,
      isLoading: isLoading ?? this.isLoading,
      hasMore: hasMore ?? this.hasMore,
      nextCursor: nextCursor ?? this.nextCursor,
      error: error,
    );
  }
}

class TransactionsNotifier extends AsyncNotifier<TransactionState> {
  @override
  Future<TransactionState> build() async {
    const state = TransactionState();
    return _load(state);
  }

  Future<TransactionState> _load(TransactionState current) async {
    final repo = ref.read(walletRepositoryProvider);
    final result =
        await repo.getTransactions(cursor: current.nextCursor);
    return result.fold(
      (failure) => current.copyWith(
        isLoading: false,
        error: failure.message,
      ),
      (transactions) => current.copyWith(
        items: [...current.items, ...transactions],
        isLoading: false,
        hasMore: transactions.length >= 20,
        nextCursor:
            transactions.isNotEmpty ? transactions.last.id : current.nextCursor,
      ),
    );
  }

  Future<void> loadMore() async {
    final current = state.valueOrNull;
    if (current == null || current.isLoading || !current.hasMore) return;
    state = AsyncData(current.copyWith(isLoading: true));
    final next = await _load(current.copyWith(isLoading: true));
    state = AsyncData(next);
  }

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => _load(const TransactionState()),
    );
  }
}

final transactionsProvider =
    AsyncNotifierProvider<TransactionsNotifier, TransactionState>(
  TransactionsNotifier.new,
);

// ---------------------------------------------------------------------------
// purchaseProvider — for buy-coins flow
// ---------------------------------------------------------------------------

class PurchaseNotifier extends StateNotifier<AsyncValue<void>> {
  PurchaseNotifier(this._repo) : super(const AsyncData(null));

  final WalletRepositoryImpl _repo;

  Future<bool> buyPackage(String packageId, WidgetRef ref) async {
    state = const AsyncLoading();
    // Step 1: create payment intent.
    final intentResult = await _repo.createPaymentIntent(packageId);
    return intentResult.fold(
      (failure) {
        state = AsyncError(failure.message, StackTrace.current);
        return false;
      },
      (intent) async {
        // Step 2: simulate payment confirmation (real app integrates Stripe SDK).
        final paymentIntentId =
            intent['payment_intent_id'] as String? ?? 'pi_simulated';
        final confirmResult = await _repo.confirmPurchase(paymentIntentId);
        return confirmResult.fold(
          (failure) {
            state = AsyncError(failure.message, StackTrace.current);
            return false;
          },
          (wallet) {
            state = const AsyncData(null);
            // Refresh the wallet balance.
            ref.invalidate(walletProvider);
            ref.invalidate(transactionsProvider);
            return true;
          },
        );
      },
    );
  }
}

// ignore: subtype_of_sealed_class
final purchaseProvider =
    StateNotifierProvider<PurchaseNotifier, AsyncValue<void>>(
  (ref) => PurchaseNotifier(ref.read(walletRepositoryProvider)),
);
