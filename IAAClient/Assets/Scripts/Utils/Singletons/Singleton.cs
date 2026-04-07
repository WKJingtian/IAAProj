using System;
using System.Reflection;
using System.Threading;

public abstract class Singleton<T> where T : Singleton<T>
{
    private static readonly Lazy<T> LazyInstance = new Lazy<T>(CreateInstance, LazyThreadSafetyMode.ExecutionAndPublication);
    private static bool _isCreating;

    public static T Instance => LazyInstance.Value;

    protected Singleton()
    {
        if (!_isCreating)
        {
            throw new InvalidOperationException($"Use {typeof(T).Name}.Instance instead of new {typeof(T).Name}().");
        }
    }

    private static T CreateInstance()
    {
        ConstructorInfo ctor = typeof(T).GetConstructor(
            BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic,
            null,
            Type.EmptyTypes,
            null);

        if (ctor == null)
        {
            throw new InvalidOperationException($"Type {typeof(T).Name} must define a parameterless constructor.");
        }

        _isCreating = true;
        try
        {
            return (T)ctor.Invoke(null);
        }
        finally
        {
            _isCreating = false;
        }
    }
}
